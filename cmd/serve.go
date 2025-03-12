package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/EO-DataHub/eodhp-workspace-services/api/handlers"
	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/api/services"
	docs "github.com/EO-DataHub/eodhp-workspace-services/docs"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/config"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title EODHP Workspace Services API
// @version v1
// @description This is the API for the EODHP Workspace Services.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the HTTP server for handling API requests",
	Run: func(cmd *cobra.Command, args []string) {

		// Load the config, initialize the database and set up logging
		commonSetUp()

		// Initialize event publisher
		publisher, err := events.NewEventPublisher(appCfg.Pulsar.URL, appCfg.Pulsar.TopicProducer)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize event publisher")
		}
		defer publisher.Close()

		// Iintialise KeyCloak client
		keycloakClient := initializeKeycloakClient(appCfg.Keycloak)

		// Create routes
		r := mux.NewRouter()

		service := &services.Service{
			Config:    appCfg,
			DB:        workspaceDB,
			Publisher: publisher,
			KC:        keycloakClient,
		}

		// Create AWS STS client
		log.Info().Str("region", appCfg.AWS.Region).Msgf("Creating AWS STS client in region '%s'...", appCfg.AWS.Region)
		sts_client := sts.New(sts.Options{
			Region: appCfg.AWS.Region,
		})

		// Register the routes
		api := r.PathPrefix(appCfg.BasePath).Subrouter()

		// Apply the middleware to the API routes
		api.Use(middleware.WithLogger)
		api.Use(middleware.JWTMiddleware)

		// Workspace routes
		api.HandleFunc("/workspaces", handlers.CreateWorkspace(service)).Methods(http.MethodPost)
		api.HandleFunc("/workspaces", handlers.GetWorkspaces(service)).Methods(http.MethodGet)
		api.HandleFunc("/workspaces/{workspace-id}", handlers.GetWorkspace(service)).Methods(http.MethodGet)
		api.HandleFunc("/workspaces/{workspace-id}", handlers.UpdateWorkspace(service)).Methods(http.MethodPut)
		api.HandleFunc("/workspaces/{workspace-id}", handlers.PatchWorkspace(service)).Methods(http.MethodPatch)

		// Workspace management routes
		api.HandleFunc("/workspaces/{workspace-id}/users", handlers.GetUsers(service)).Methods(http.MethodGet)
		api.HandleFunc("/workspaces/{workspace-id}/users/{username}", handlers.AddUser(service)).Methods(http.MethodPut)
		api.HandleFunc("/workspaces/{workspace-id}/users/{username}", handlers.GetUser(service)).Methods(http.MethodGet)
		api.HandleFunc("/workspaces/{workspace-id}/users/{username}", handlers.RemoveUser(service)).Methods(http.MethodDelete)

		// Account routes
		api.HandleFunc("/accounts", handlers.CreateAccount(service)).Methods(http.MethodPost)
		api.HandleFunc("/accounts", handlers.GetAccounts(service)).Methods(http.MethodGet)
		api.HandleFunc("/accounts/{account-id}", handlers.GetAccount(service)).Methods(http.MethodGet)
		api.HandleFunc("/accounts/{account-id}", handlers.DeleteAccount(service)).Methods(http.MethodDelete)
		api.HandleFunc("/accounts/{account-id}", handlers.UpdateAccount(service)).Methods(http.MethodPut)

		// S3 token routes
		api.HandleFunc("/workspaces/{workspace-id}/{user-id}/s3-tokens", handlers.RequestS3CredentialsHandler(appCfg.AWS.S3.RoleArn, sts_client, *keycloakClient)).Methods(http.MethodPost)

		// Linked account routes
		api.HandleFunc("/workspaces/{workspace-id}/linked-accounts", handlers.CreateLinkedAccount(service)).Methods(http.MethodPost)
		api.HandleFunc("/workspaces/{workspace-id}/linked-accounts", handlers.GetLinkedAccounts(service)).Methods(http.MethodGet)
		api.HandleFunc("/workspaces/{workspace-id}/linked-accounts/{provider}", handlers.DeleteLinkedAccount(service)).Methods(http.MethodDelete)

		// Docs
		docs.SwaggerInfo.Host = appCfg.Host
		docs.SwaggerInfo.BasePath = appCfg.BasePath
		r.PathPrefix(appCfg.DocsPath).Handler(httpSwagger.Handler(
			httpSwagger.URL(path.Join(appCfg.DocsPath, "/doc.json")),
			httpSwagger.DeepLinking(true),
			httpSwagger.DocExpansion("none"),
			httpSwagger.DomID("swagger-ui"),
		)).Methods(http.MethodGet)

		log.Info().Msg(fmt.Sprintf("Server started at %s:%d", host, port))

		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port),
			r); err != nil {

			log.Error().Err(err).Msg("could not start server")
		}
		log.Info().Msg(fmt.Sprintf("Server running on http://%s:%d", host, port))
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVar(&host, "host", "0.0.0.0", "host to run the server on")
	serveCmd.Flags().IntVar(&port, "port", 8080, "port to run the server on")

}

// InitializeKeycloakClient initializes the Keycloak client and retrieves the access token.
func initializeKeycloakClient(kcCfg config.KeycloakConfig) *services.KeycloakClient {
	keycloakClientSecret := os.Getenv("KEYCLOAK_CLIENT_SECRET")

	// Create a new Keycloak client
	keycloakClient := services.NewKeycloakClient(kcCfg.URL, kcCfg.ClientId, keycloakClientSecret, kcCfg.Realm)

	return keycloakClient
}
