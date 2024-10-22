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

		setUp()

		// Ensure the database and notification connections close if gracefully program exits
		defer func() {
			if workspaceDB != nil {
				err := workspaceDB.Close()
				if err != nil {
					log.Fatal().Err(err).Msg("Failed to close database connection")
				}

			}
		}()

		// Create routes
		r := mux.NewRouter()

		// The CRUD routes all use the same middleware
		var middleware = func(next http.HandlerFunc) http.HandlerFunc {
			return middleware.WithLogger(middleware.JWTMiddleware(next))
		}

		// Register the routes

		// s3 routes
		r.HandleFunc("/api/workspaces/s3/credentials", middleware(handlers.GetS3Credentials())).Methods(http.MethodGet)

		// workspace routes
		r.HandleFunc("/api/workspaces", middleware(handlers.CreateWorkspace(workspaceDB))).Methods(http.MethodPost)
		r.HandleFunc("/api/workspaces", middleware(handlers.GetWorkspaces(workspaceDB))).Methods(http.MethodGet)
		r.HandleFunc("/api/workspaces/{workspace-id}", middleware(handlers.UpdateWorkspace(workspaceDB))).Methods(http.MethodPut)
		r.HandleFunc("/api/workspaces/{workspace-id}", middleware(handlers.PatchWorkspace(workspaceDB))).Methods(http.MethodPatch)

		log.Info().Msg(fmt.Sprintf("Server started at %s:%d", host, port))

		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port),
			r); err != nil {

			log.Error().Err(err).Msg("could not start server")
		}
		log.Info().Msg(fmt.Sprintf("Server running on http://localhost:%d", port))
		//log.Printf("Server is running on http://localhost:%d", port)
	},
}

func init() {
	rootCmd.AddCommand(runServerCmd)
	runServerCmd.Flags().StringVar(&host, "host", "0.0.0.0", "host to run the server on")
	runServerCmd.Flags().IntVar(&port, "port", 8080, "port to run the server on")

}
