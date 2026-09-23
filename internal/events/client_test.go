package events

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/EO-DataHub/eodhp-workspace-services/internal/appconfig"
	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/stretchr/testify/require"
)

func writeToken(t *testing.T, token string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "TOKEN")
	require.NoError(t, os.WriteFile(path, []byte(token), 0o600))
	return path
}

func TestAuthenticationAnonymous(t *testing.T) {
	t.Parallel()

	auth, err := authentication("")
	require.NoError(t, err)
	require.Nil(t, auth)
}

func TestAuthenticationMissingTokenFile(t *testing.T) {
	t.Parallel()

	missing := filepath.Join(t.TempDir(), "TOKEN")
	_, err := authentication(missing)
	require.ErrorContains(t, err, "pulsar tokenFile is set but cannot be read")
	require.ErrorContains(t, err, missing)
}

func TestNewClientWithTokenFile(t *testing.T) {
	t.Parallel()

	// The client reads the token when it is created, without connecting.
	client, err := newClient(appconfig.PulsarConfig{
		URL:       "pulsar://localhost:6650",
		TokenFile: writeToken(t, "header.payload.signature\n"),
	})
	require.NoError(t, err)
	client.Close()
}

func TestNewClientWithEmptyTokenFile(t *testing.T) {
	t.Parallel()

	_, err := newClient(appconfig.PulsarConfig{
		URL:       "pulsar://localhost:6650",
		TokenFile: writeToken(t, ""),
	})
	require.ErrorContains(t, err, "could not create Pulsar client")
}

func TestNewEventConsumerRequiresTopic(t *testing.T) {
	t.Parallel()

	_, err := NewEventConsumer(appconfig.PulsarConfig{URL: "pulsar://localhost:6650", TopicConsumer: " , "})
	require.EqualError(t, err, "no Pulsar consumer topic configured")
}

func TestConsumerOptions(t *testing.T) {
	t.Parallel()

	topics := []string{
		"persistent://public/workspaces/workspace-status",
		"persistent://public/default/workspace-status",
	}
	opts := consumerOptions(topics, "workspace-status-sub")

	require.Empty(t, opts.Topic)
	require.Equal(t, topics, opts.Topics)
	require.Equal(t, "workspace-status-sub", opts.SubscriptionName)
	require.Equal(t, pulsar.Shared, opts.Type)
	require.Equal(t, uint32(3), opts.DLQ.MaxDeliveries)
	require.Equal(t, "persistent://public/workspaces/workspace-status-dlq", opts.DLQ.DeadLetterTopic)
}

func TestConsumerOptionsSingleTopic(t *testing.T) {
	t.Parallel()

	opts := consumerOptions([]string{"persistent://public/default/workspace-status"}, "workspace-status-sub")
	require.Equal(t, "persistent://public/default/workspace-status-dlq", opts.DLQ.DeadLetterTopic)
}
