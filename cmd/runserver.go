package cmd

import (
	"fmt"
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/handlers"
	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var runServerCmd = &cobra.Command{
	Use:   "runserver",
	Short: "Run the server",
	Long:  `Run the workspace services server`,
	Run: func(cmd *cobra.Command, args []string) {

		// Init the logging
		setUp()
		// Create routes
		r := mux.NewRouter()

		// The CRUD routes all use the same middleware
		var middleware = func(next http.HandlerFunc) http.HandlerFunc {
			return middleware.WithLogger(middleware.JWTMiddleware(next))
		}

		// Register the routes
		r.HandleFunc("/api/workspaces/s3/credentials", middleware(handlers.GetS3Credentials())).Methods(http.MethodGet)
		r.HandleFunc("/api/workspaces/workspace/create", middleware(handlers.CreateWorkspace())).Methods(http.MethodPost)
		log.Info().Msg(fmt.Sprintf("Server started at %s:%d", host, port))

		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port),
			r); err != nil {

			log.Error().Err(err).Msg("could not start server")
		}

		log.Printf("Server is running on http://localhost:%d", port)

		// Ensure Pulsar is closed on exit
		defer events.ClosePublisher()

	},
}

func init() {
	rootCmd.AddCommand(runServerCmd)
	runServerCmd.Flags().StringVar(&host, "hos	t", "0.0.0.0", "host to run the server on")
	runServerCmd.Flags().IntVar(&port, "port", 8080, "port to run the server on")
	runServerCmd.Flags().StringVar(&configPath, "config", "", "Path to the configuration YAML file")

}
