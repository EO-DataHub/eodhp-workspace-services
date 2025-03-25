package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"

	"github.com/EO-DataHub/eodhp-workspace-services/api/handlers"
	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/api/services"
	docs "github.com/EO-DataHub/eodhp-workspace-services/docs"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	httpSwagger "github.com/swaggo/http-swagger"
)

// @title EODHP Workspace Services API
// @version v1
// @description This is the API for the EODHP Workspace Services.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the HTTP server for handling API requests",
	Run: func(cmd *cobra.Command, args []string) {

		// Load the config, initialize the database, keycloak, AWS Secrets Manager and set up logging
		commonSetUp()

		// Initialize event publisher
		publisher, err := events.NewEventPublisher(appCfg.Pulsar.URL, appCfg.Pulsar.TopicProducer)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize event publisher")
		}
		defer publisher.Close()

		// Initialize Kubernetes client
		k8sClient, err := initializeK8sClient()
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize Kubernetes client")
		}

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
		api.HandleFunc("/workspaces/{workspace-id}", handlers.DeleteWorkspace(service)).Methods(http.MethodDelete)

		// Workspace management routes
		api.HandleFunc("/workspaces/{workspace-id}/users", handlers.GetUsers(service)).Methods(http.MethodGet)
		api.HandleFunc("/workspaces/{workspace-id}/users/{username}", handlers.AddUser(service)).Methods(http.MethodPut)
		api.HandleFunc("/workspaces/{workspace-id}/users/{username}", handlers.GetUser(service)).Methods(http.MethodGet)
		api.HandleFunc("/workspaces/{workspace-id}/users/{username}", handlers.RemoveUser(service)).Methods(http.MethodDelete)

		// Account routes
		// Deny workspace-scoped tokens for /accounts routes
		// Initialize secrets manager client
		awsEmailClient, err = InitializeAWSEMailClient(appCfg.AWS.Region)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize AWS SES client")
		}
		// Linked account routes
		billingAccountService := &services.BillingAccountService{
			Config:         appCfg,
			DB:             workspaceDB,
			AWSEmailClient: awsEmailClient,
		}
		accountRouter := api.PathPrefix("/accounts").Subrouter()
		accountRouter.Use(middleware.DenyWorkspaceScopedTokens)

		accountRouter.HandleFunc("", handlers.CreateAccount(billingAccountService)).Methods(http.MethodPost)
		accountRouter.HandleFunc("", handlers.GetAccounts(billingAccountService)).Methods(http.MethodGet)
		accountRouter.HandleFunc("/{account-id}", handlers.GetAccount(billingAccountService)).Methods(http.MethodGet)
		accountRouter.HandleFunc("/{account-id}", handlers.DeleteAccount(billingAccountService)).Methods(http.MethodDelete)
		accountRouter.HandleFunc("/{account-id}", handlers.UpdateAccount(billingAccountService)).Methods(http.MethodPut)

		accountAdminRouter := accountRouter.PathPrefix("/admin").Subrouter()
		accountAdminRouter.Use(middleware.WithLogger)
		accountAdminRouter.Use(middleware.JWTMiddleware)
		accountAdminRouter.HandleFunc("/approve/{token}", handlers.AccountStatusHandler(billingAccountService, services.AccountStatusApproved)).Methods(http.MethodGet)
		accountAdminRouter.HandleFunc("/deny/{token}", handlers.AccountStatusHandler(billingAccountService, services.AccountStatusDenied)).Methods(http.MethodGet)

		// Workspace scoped session routes
		api.HandleFunc("/workspaces/{workspace-id}/{user-id}/sessions", handlers.CreateWorkspaceSession(keycloakClient)).Methods(http.MethodPost)

		// S3 token routes
		api.HandleFunc("/workspaces/{workspace-id}/{user-id}/s3-tokens", handlers.RequestS3CredentialsHandler(appCfg.AWS.S3.RoleArn, sts_client, *keycloakClient)).Methods(http.MethodPost)

		// Linked account routes
		linkedAccountService := &services.LinkedAccountService{
			DB:             workspaceDB,
			SecretsManager: secretsManagerClient,
			K8sClient:      k8sClient,
		}
		api.HandleFunc("/workspaces/{workspace-id}/linked-accounts", handlers.CreateLinkedAccount(linkedAccountService)).Methods(http.MethodPost)
		api.HandleFunc("/workspaces/{workspace-id}/linked-accounts", handlers.GetLinkedAccounts(linkedAccountService)).Methods(http.MethodGet)
		api.HandleFunc("/workspaces/{workspace-id}/linked-accounts/{provider}", handlers.DeleteLinkedAccount(linkedAccountService)).Methods(http.MethodDelete)

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

func initializeK8sClient() (*kubernetes.Clientset, error) {
	var config *rest.Config
	var err error

	// Check if running inside a Kubernetes pod
	if _, exists := os.LookupEnv("KUBERNETES_SERVICE_HOST"); exists {
		// Inside Kubernetes, use in-cluster config
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load in-cluster Kubernetes config: %v", err)
		}
	} else {
		// Running locally, use kubeconfig
		config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %v", err)
		}
	}

	// Create Kubernetes client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	return clientset, nil
}

// InitializeSESClient creates and returns a new SESClient instance with options
func InitializeAWSEMailClient(region string) (*sesv2.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, err
	}

	client := sesv2.NewFromConfig(cfg)
	return client, nil
}
