package cmd

import (
	"fmt"
	"net/http"

	"github.com/EO-DataHub/eodhp-workspace-services/api/handlers"
	"github.com/EO-DataHub/eodhp-workspace-services/api/middleware"
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

		log.Info().Msg(fmt.Sprintf("Server started at %s:%d", host, port))
		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port),
			r); err != nil {

			log.Error().Err(err).Msg("could not start server")
		}
	},
}

func init() {
	rootCmd.AddCommand(runServerCmd)
	runServerCmd.Flags().StringVar(&host, "host", "0.0.0.0", "host to run the server on")
	runServerCmd.Flags().IntVar(&port, "port", 8080, "port to run the server on")
}
