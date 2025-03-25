package appconfig

import (
	"bytes"
	"errors"
	"html/template"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v2"
)

// Config holds all configuration details
type Config struct {
	Host     string         `yaml:"host"`
	BasePath string         `yaml:"basePath"`
	DocsPath string         `yaml:"docsPath"`
	Accounts AccountsConfig `yaml:"accounts"`
	Database DatabaseConfig `yaml:"database"`
	Pulsar   PulsarConfig   `yaml:"pulsar"`
	Keycloak KeycloakConfig `yaml:"keycloak"`
	AWS      AWSConfig      `yaml:"aws"`
}

// AccountsConfig defines the email chain for account approval requests
type AccountsConfig struct {
	ServiceAccountEmail string `yaml:"serviceAccountEmail"`
	HelpdeskEmail       string `yaml:"helpdeskEmail"`

}

// DatabaseConfig defines the database connection details
type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	Source string `yaml:"source"`
}

// PulsarConfig defines the messaging system connection details
type PulsarConfig struct {
	URL           string `yaml:"url"`
	TopicProducer string `yaml:"topicProducer"`
	TopicConsumer string `yaml:"topicConsumer"`
	Subscription  string `yaml:"subscription"`
}

// KeycloakConfig defines authentication configuration
type KeycloakConfig struct {
	ClientId string `yaml:"clientId"`
	URL      string `yaml:"url"`
	Realm    string `yaml:"realm"`
}

type S3Config struct {
	Bucket  string `yaml:"bucket"`
	Host    string `yaml:"host"`
	RoleArn string `yaml:"roleArn"`
}

type AWSConfig struct {
	Account         string   `yaml:"account"`
	Region          string   `yaml:"region"`
	WorkspaceDomain string   `yaml:"workspace_domain"`
	S3              S3Config `yaml:"s3"`
}

// LoadConfig loads and parses the configuration from a given file path
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		err := errors.New("config file path is required")
		log.Fatal().Err(err).Msg("config file not provided")
		return nil, err
	}

	// Parse the template file
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		log.Fatal().Err(err).Msg("error parsing config file template")
		return nil, err
	}

	// Create a map of environment variables
	envVars := loadEnvVars()

	// Execute the template with environment variables
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, envVars)
	if err != nil {
		log.Fatal().Err(err).Msg("error executing config file template")
		return nil, err
	}

	// Load and unmarshal the YAML
	var config Config
	if err := yaml.Unmarshal(buf.Bytes(), &config); err != nil {
		log.Fatal().Err(err).Msg("failed to unmarshal config YAML")
		return nil, err
	}

	return &config, nil
}

// loadEnvVars loads environment variables into a map
func loadEnvVars() map[string]string {
	envVars := make(map[string]string)
	for _, env := range os.Environ() {
		kv := strings.SplitN(env, "=", 2)
		if len(kv) == 2 {
			envVars[kv[0]] = kv[1]
		}
	}
	return envVars
}
