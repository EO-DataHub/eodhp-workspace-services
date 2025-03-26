package cmd

import (
	"fmt"
	"os"

	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "init-db-migrate",
	Short: "Initialize tables and run database migrations",
	Long:  `This job ensures tables exist and then runs goose migrations.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Set the log level
		setLogging(logLevel)

		// Load the config file
		var err error
		appCfg, err = appconfig.LoadConfig(configPath)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to load config")
		}

		err = os.Setenv("DATABASE_URL", appCfg.Database.Source)
		if err != nil {
			fmt.Println("Error setting environment variable:", err)
			os.Exit(1)
		}

		workspaceDB, err = db.NewWorkspaceDB(appCfg.AWS)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize WorkspaceDB")
			os.Exit(1)
		}

		// Set up the database
		defer workspaceDB.Close()

		// Run the migrations
		log.Info().Msgf("Running migrations...")
		if err := workspaceDB.Migrate(); err != nil {
			log.Fatal().Err(err).Msg("Failed to run migrations")
		}

		log.Info().Msg("Migrations complete")
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
