package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/EO-DataHub/eodhp-workspace-services/api/handlers"
	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/api/services"
	"github.com/EO-DataHub/eodhp-workspace-services/aws"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/config"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

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

		// The CRUD routes all use the same middleware
		var middleware = func(next http.HandlerFunc) http.HandlerFunc {
			return middleware.WithLogger(middleware.JWTMiddleware(next))
		}

		service := &services.Service{
			Config:    appCfg,
			DB:        workspaceDB,
			Publisher: publisher,
			KC:        keycloakClient,
		}

		// Instance of real AWS STS Client
		stsClient := &aws.DefaultSTSClient{}

		// Register the routes
		r.HandleFunc("/api/workspaces/s3/credentials", middleware(handlers.GetS3Credentials(stsClient))).Methods(http.MethodGet)

		// Workspace routes
		r.HandleFunc("/api/workspaces", middleware(handlers.CreateWorkspace(service))).Methods(http.MethodPost)
		r.HandleFunc("/api/workspaces", middleware(handlers.GetWorkspaces(service))).Methods(http.MethodGet)
		r.HandleFunc("/api/workspaces/{workspace-id}", middleware(handlers.GetWorkspace(service))).Methods(http.MethodGet)
		r.HandleFunc("/api/workspaces/{workspace-id}", middleware(handlers.UpdateWorkspace(service))).Methods(http.MethodPut)
		r.HandleFunc("/api/workspaces/{workspace-id}", middleware(handlers.PatchWorkspace(service))).Methods(http.MethodPatch)

		// Workspace management routes
		r.HandleFunc("/api/workspaces/{workspace-id}/users", middleware(handlers.GetUsers(service))).Methods(http.MethodGet)
		r.HandleFunc("/api/workspaces/{workspace-id}/users/{username}", middleware(handlers.AddUser(service))).Methods(http.MethodPut)
		r.HandleFunc("/api/workspaces/{workspace-id}/users/{username}", middleware(handlers.GetUser(service))).Methods(http.MethodGet)
		r.HandleFunc("/api/workspaces/{workspace-id}/users/{username}", middleware(handlers.RemoveUser(service))).Methods(http.MethodDelete)

		// Account routes
		r.HandleFunc("/api/accounts", middleware(handlers.CreateAccount(service))).Methods(http.MethodPost)
		r.HandleFunc("/api/accounts", middleware(handlers.GetAccounts(service))).Methods(http.MethodGet)
		r.HandleFunc("/api/accounts/{account-id}", middleware(handlers.GetAccount(service))).Methods(http.MethodGet)
		r.HandleFunc("/api/accounts/{account-id}", middleware(handlers.DeleteAccount(service))).Methods(http.MethodDelete)
		r.HandleFunc("/api/accounts/{account-id}", middleware(handlers.UpdateAccount(service))).Methods(http.MethodPut)

		log.Info().Msg(fmt.Sprintf("Server started at %s:%d", host, port))

		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port),
			r); err != nil {

			log.Error().Err(err).Msg("could not start server")
		}
		log.Info().Msg(fmt.Sprintf("Server running on http://localhost:%d", port))
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
