package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"os"
	"strings"
	"time"

	"github.com/EO-DataHub/eodhp-workspace-services/db"
	"github.com/EO-DataHub/eodhp-workspace-services/internal/events"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var (
	logLevel    string
	host        string
	port        int
	configPath  string
	config      *Config
	workspaceDB *db.WorkspaceDB
)

type Config struct {
	Database databaseConfig `yaml:"database"`
	Pulsar   pulsarConfig   `yaml:"pulsar"`
}

type databaseConfig struct {
	Driver string `yaml:"driver"`
	Source string `yaml:"source"`
}

type pulsarConfig struct {
	URL string `yaml:"url"`
}

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
	rootCmd.PersistentFlags().StringVar(&logLevel, "log", "warn", "sets the log level")
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "/etc/workspace-services/config.yaml", "path to config file")
}

func setUp() {

	setLogging(logLevel)

	// Load the config file
	var err error
	config, err = loadConfig(configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	// Initialize Pulsar connection
	var eventPublisher *events.EventPublisher
	if eventPublisher, err = initializeNotifications(&config.Pulsar); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize event notifier")
	}

	// Initialize WorkspaceDB
	err = initializeDatabase(eventPublisher)
	if err != nil {
		fmt.Println("Failed to initialize database", err)
		return
	}

}

func initializeDatabase(eventPublisher *events.EventPublisher) error {

	err := os.Setenv("DATABASE_URL", config.Database.Source)
	if err != nil {
		fmt.Println("Error setting environment variable:", err)
		return err
	}

	workspaceDB, err = db.NewWorkspaceDB(eventPublisher, &log.Logger)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize WorkspaceDB")
		return err
	}

	// Create database tables if they don't exist
	err = workspaceDB.InitTables()
	if err != nil {
		workspaceDB.Log.Fatal().Err(err).Msg("Failed to initialize database tables")
	}

	return nil
}

func initializeNotifications(config *pulsarConfig) (*events.EventPublisher, error) {

	topic := "persistent://public/default/workspaces-services"
	ackTopic := "persistent://public/default/workspace-ack-topic"
	eventPublisher, err := events.NewEventPublisher(config.URL, topic, ackTopic)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Pulsar event publisher")
	}

	return eventPublisher, nil

}

func setLogging(level string) zerolog.Logger {
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
	logger := zerolog.New(consoleWriter).With().Timestamp().Logger()

	return logger
}

func loadConfig(path string) (*Config, error) {
	// Read the config file
	if path == "" {
		err := errors.New("--config flag is required")
		log.Fatal().Err(err).Msg("config file not provided")
		return nil, err
	}

	// Parse the template file
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		log.Fatal().Err(err).Msg("error parsing config file template")
	}

	// Create a map of environment variables
	envVars := loadEnvVars()

	// Execute the template with the environment variables
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, envVars)
	if err != nil {
		log.Fatal().Err(err).Msg("error executing config file template")
	}

	// Load the config
	c := &Config{}
	if err := c.loadConfig(buf.Bytes()); err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
		return nil, err
	}

	return c, nil
}

func (c *Config) loadConfig(data []byte) error {
	// Unmarshal the YAML data into the config struct
	err := yaml.Unmarshal(data, &c)
	if err != nil {
		log.Error().Err(err).Msg("failed to unmarshal file")
		return err
	}

	return nil
}

func loadEnvVars() map[string]string {
	envVars := make(map[string]string)
	for _, env := range os.Environ() {
		kv := strings.Split(env, "=")
		if len(kv) == 2 {
			envVars[kv[0]] = kv[1]
		}
	}
	return envVars
}
