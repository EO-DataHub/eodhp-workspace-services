package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

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

		if tunnelConfigFile != "" {

			// Load SSH configuration from the JSON file
			tunnelConfig, err := loadTunnelConfig(tunnelConfigFile)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to load SSH tunnel config: %v", err)
			}

			go func() {
				err := StartSSHTunnel(tunnelConfig)
				if err != nil {
					log.Error().Err(err).Msgf("Failed to start SSH tunnel: %v", err)
					return
				}
				log.Info().Msg("SSH tunnel started successfully")
			}()
		}

		// Register the routes
		r.HandleFunc("/api/workspaces/s3/credentials", middleware(handlers.GetS3Credentials())).Methods(http.MethodGet)
		r.HandleFunc("/api/workspaces/workspace/create", middleware(handlers.CreateWorkspace())).Methods(http.MethodGet)
		log.Info().Msg(fmt.Sprintf("Server started at %s:%d", host, port))
		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port),
			r); err != nil {

			log.Error().Err(err).Msg("could not start server")
		}

		log.Printf("Server is running on http://localhost:%d", port)

	},
}

func init() {
	rootCmd.AddCommand(runServerCmd)
	runServerCmd.Flags().StringVar(&host, "host", "0.0.0.0", "host to run the server on")
	runServerCmd.Flags().IntVar(&port, "port", 8080, "port to run the server on")
	//runServerCmd.Flags().BoolVar(&tunnel, "tunnel", false, "tSSH tunnel to access the database locally")
	runServerCmd.Flags().StringVar(&tunnelConfigFile, "tunnel-config-file", "", "Path to the SSH tunnel configuration JSON file")

}

// loadSSHConfig loads SSH configuration from a JSON file
func loadTunnelConfig(filepath string) (*TunnelConfig, error) {
	// Open the JSON file
	file, err := os.Open(filepath) // Provide the correct path to your JSON file
	if err != nil {
		return nil, fmt.Errorf("could not open file: %w", err)
	}
	defer file.Close()

	// Decode the JSON file into the SSHConfig struct
	var config TunnelConfig
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("could not decode JSON: %w", err)
	}

	// Validate the required fields
	if config.SSHUser == "" || config.SSHHost == "" || config.PrivateKeyPath == "" ||
		config.RemoteHost == "" || config.RemotePort == "" || config.LocalPort == "" {
		return nil, fmt.Errorf("incomplete SSH config: missing required fields")
	}

	// Print the SSHConfig to verify
	fmt.Printf("Loaded SSH Config: %+v\n", config)

	return &config, nil
}
