/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"encoding/json"
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
	logLevel         string
	host             string
	port             int
	configPath       string
	tunnelConfigFile string
	config           *Config
)

type Config struct {
	Database databaseConfig `yaml:"database"`
}

type databaseConfig struct {
	Driver string `yaml:"driver"`
	Source string `yaml:"source"`
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
	rootCmd.PersistentFlags().StringVar(&logLevel, "log", "warn",
		"sets the log level")
	rootCmd.PersistentFlags().StringVar(&configPath, "config",
		"/etc/workspace-services/config.yaml", "path to config file")
}

func setUp() {
	setLogging(logLevel)

	// Initialize Pulsar connection
	if err := setupPulsar(); err != nil {
		fmt.Println("Failed to initialize Pulsar")
	}

	// initialize the database, including tunneling if needed
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
	time.Sleep(3 * time.Second)

	// Load the config file
	var err error
	config, err = loadConfig(configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	// Set the DATABASE_URL environment variable
	err = os.Setenv("DATABASE_URL", config.Database.Source)
	if err != nil {
		fmt.Println("Error setting environment variable:", err)
		return
	}
	fmt.Println("config loaded")
	bla := os.Getenv("DATABASE_URL")
	fmt.Println("ENV ", bla)
	fmt.Printf("database driver: %s\n", config.Database.Driver)
	fmt.Printf("database source: %s\n", config.Database.Source)
	// Initialize database tables if they dont exist
	db.InitTables()

	// load the config file
}

// loadSSHConfig loads SSH configuration from a JSON file
func loadTunnelConfig(filepath string) (*TunnelConfig, error) {
	// Open the JSON file
	file, err := os.Open(filepath)
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

func setupPulsar() error {
	pulsarURL := "pulsar://localhost:6650" // Example Pulsar URL, can also be configurable
	topic := "persistent://public/default/workspaces-services"

	// Initialize Pulsar event publisher
	err := events.InitEventPublisher(pulsarURL, topic)
	if err != nil {
		return fmt.Errorf("failed to initialize Pulsar: %w", err)
	}
	fmt.Println("Pulsar event publisher initialized successfully")
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
