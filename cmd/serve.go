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

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the HTTP server for handling API requests",
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
		r.HandleFunc("/api/workspaces/s3/credentials", middleware(handlers.GetS3Credentials())).Methods(http.MethodGet)

		// Workspace routes
		r.HandleFunc("/api/workspaces", middleware(handlers.CreateWorkspace(workspaceDB))).Methods(http.MethodPost)
		r.HandleFunc("/api/workspaces", middleware(handlers.GetWorkspaces(workspaceDB))).Methods(http.MethodGet)
		r.HandleFunc("/api/workspaces/{workspace-id}", middleware(handlers.UpdateWorkspace(workspaceDB))).Methods(http.MethodPut)
		r.HandleFunc("/api/workspaces/{workspace-id}", middleware(handlers.PatchWorkspace(workspaceDB))).Methods(http.MethodPatch)

		// Account routes
		r.HandleFunc("/api/accounts", middleware(handlers.CreateAccount(workspaceDB))).Methods(http.MethodPost)
		r.HandleFunc("/api/accounts", middleware(handlers.GetAccounts(workspaceDB))).Methods(http.MethodGet)
		r.HandleFunc("/api/accounts/{account-id}", middleware(handlers.GetAccount(workspaceDB))).Methods(http.MethodGet)
		r.HandleFunc("/api/accounts/{account-id}", middleware(handlers.DeleteAccount(workspaceDB))).Methods(http.MethodDelete)
		r.HandleFunc("/api/accounts/{account-id}", middleware(handlers.UpdateAccount(workspaceDB))).Methods(http.MethodPut)

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
