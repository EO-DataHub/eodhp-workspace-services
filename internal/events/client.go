package events

import (
	"fmt"
	"os"

	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	"github.com/apache/pulsar-client-go/pulsar"
)

// authentication returns token authentication read from tokenFile, or nil
// (anonymous) when tokenFile is empty. The client re-reads the file on every
// connection, so a rotated token is picked up without a restart.
func authentication(tokenFile string) (pulsar.Authentication, error) {
	if tokenFile == "" {
		return nil, nil
	}
	if _, err := os.Stat(tokenFile); err != nil {
		return nil, fmt.Errorf("pulsar tokenFile is set but cannot be read: %w", err)
	}
	return pulsar.NewAuthenticationTokenFromFile(tokenFile), nil
}

// newClient creates a Pulsar client for cfg.
func newClient(cfg appconfig.PulsarConfig) (pulsar.Client, error) {
	auth, err := authentication(cfg.TokenFile)
	if err != nil {
		return nil, err
	}

	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL:            cfg.URL,
		Authentication: auth,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create Pulsar client: %w", err)
	}
	return client, nil
}
