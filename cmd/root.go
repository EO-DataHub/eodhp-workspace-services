package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/config"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

var (
	logLevel    string
	host        string
	port        int
	configPath  string
	appCfg      *config.Config
	workspaceDB *db.WorkspaceDB
	logger      zerolog.Logger
)

var rootCmd = &cobra.Command{
	Use:   "workspace-services",
	Short: "Workspace Services",
	Long:  `Workspace Services is a CLI tool for managing platform access to Workspace utilities externally.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&logLevel, "log", "info", "sets the log level")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "/etc/workspace-services/config.yaml", "path to config file")

}

func commonSetUp() {

	setLogging(logLevel)

	// Load the config file
	var err error
	appCfg, err = config.LoadConfig(configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	// Initialize WorkspaceDB
	err = initializeDatabase()
	if err != nil {
		fmt.Println("Failed to initialize database", err)
		return
	}
}

func initializeDatabase() error {

	err := os.Setenv("DATABASE_URL", appCfg.Database.Source)
	if err != nil {
		fmt.Println("Error setting environment variable:", err)
		return err
	}

	workspaceDB, err = db.NewWorkspaceDB(appCfg.AWS)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize WorkspaceDB")
		return err
	}

	// Create database tables if they don't exist
	err = workspaceDB.InitTables()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database tables")
	}

	return nil
}

func setLogging(level string) {
	zerolog.TimestampFunc = func() time.Time {
		return time.Now().UTC()
	}

	switch strings.ToLower(level) {
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case "info":
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case "warn":
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "fatal":
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case "panic":
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	}

	// Initialize logger
	consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout}
	zerolog.New(consoleWriter).With().Timestamp().Logger()
}
