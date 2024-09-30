/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
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
	logLevel   string
	host       string
	port       int
	configPath string
	config     *Config
)

type Config struct {
	Database      databaseConfig `yaml:"database"`
	DatabaseProxy databaseProxy  `yaml:"databaseProxy"`
}

type databaseConfig struct {
	Driver string `yaml:"driver"`
	Source string `yaml:"source"`
}

type databaseProxy struct {
	SSHUser        string `yaml:"sshUser"`
	SSHHost        string `yaml:"sshHost"`
	SSHPort        string `yaml:"sshPort"`
	RemoteHost     string `yaml:"remoteHost"`
	RemotePort     string `yaml:"reportPort"`
	LocalPort      string `yaml:"localPort"`
	PrivateKeyPath string `yaml:"privateKeyPath"`
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

	// Load the config file
	var err error
	config, err = loadConfig(configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	// If you want to connect to the db from development VM
	if config.DatabaseProxy.RemoteHost != "" {
		go func() {
			err := StartSSHTunnel(&config.DatabaseProxy)
			if err != nil {
				log.Error().Err(err).Msgf("Failed to start SSH tunnel: %v", err)
				return
			}
			log.Info().Msg("SSH tunnel started successfully")
		}()

		time.Sleep(3 * time.Second)
	}

	// Set the DATABASE_URL environment variable
	err = os.Setenv("DATABASE_URL", config.Database.Source)
	if err != nil {
		fmt.Println("Error setting environment variable:", err)
		return
	}
	fmt.Println("config loaded")
	fmt.Printf("database driver: %s\n", config.Database.Driver)
	fmt.Printf("database source: %s\n", config.Database.Source)

	// Initialize database tables if they dont exist
	err = db.InitTables()

	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize database tables")
	}

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
