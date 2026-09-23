package appconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const pulsarConfigTemplate = `pulsar:
  url: pulsar://pulsar-proxy.pulsar:6650
  topicProducer: persistent://public/default/workspace-settings
  topicConsumer: {{ .TEST_TOPIC_CONSUMER }}
  subscription: workspace-status-sub
  tokenFile: "{{ .PULSAR_TOKEN_FILE }}"
`

func loadPulsarConfig(t *testing.T) PulsarConfig {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(pulsarConfigTemplate), 0o600))
	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	return cfg.Pulsar
}

func TestLoadConfigPulsarTokenFile(t *testing.T) {
	t.Setenv("TEST_TOPIC_CONSUMER", "persistent://public/default/workspace-status")
	t.Setenv("PULSAR_TOKEN_FILE", "/var/run/secrets/pulsar-token/TOKEN")

	cfg := loadPulsarConfig(t)
	require.Equal(t, "/var/run/secrets/pulsar-token/TOKEN", cfg.TokenFile)
	require.Equal(t, []string{"persistent://public/default/workspace-status"}, cfg.ConsumerTopics())
}

func TestLoadConfigPulsarTokenFileUnset(t *testing.T) {
	t.Setenv("TEST_TOPIC_CONSUMER", "persistent://public/default/workspace-status")
	t.Setenv("PULSAR_TOKEN_FILE", "")
	require.NoError(t, os.Unsetenv("PULSAR_TOKEN_FILE"))

	// html/template renders a missing key as an empty string, so the client
	// stays anonymous.
	cfg := loadPulsarConfig(t)
	require.Empty(t, cfg.TokenFile)
}

func TestLoadConfigPulsarTopicList(t *testing.T) {
	t.Setenv("TEST_TOPIC_CONSUMER", "persistent://public/default/workspace-status,persistent://public/workspaces/workspace-status")
	t.Setenv("PULSAR_TOKEN_FILE", "")

	cfg := loadPulsarConfig(t)
	require.Equal(t, []string{
		"persistent://public/default/workspace-status",
		"persistent://public/workspaces/workspace-status",
	}, cfg.ConsumerTopics())
}

func TestConsumerTopics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "empty", value: "", want: nil},
		{name: "single", value: "persistent://public/default/a", want: []string{"persistent://public/default/a"}},
		{
			name:  "list with spaces",
			value: "persistent://public/default/a, persistent://public/workspaces/a",
			want:  []string{"persistent://public/default/a", "persistent://public/workspaces/a"},
		},
		{
			name:  "empty entries",
			value: ",persistent://public/default/a,,persistent://public/workspaces/a,",
			want:  []string{"persistent://public/default/a", "persistent://public/workspaces/a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, PulsarConfig{TopicConsumer: tt.value}.ConsumerTopics())
		})
	}
}
