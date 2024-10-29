package events

import (
	"context"
	"encoding/json"
	"fmt"

	"time"

	"github.com/EO-DataHub/eodhp-workspace-services/models"
	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/rs/zerolog/log"
)

type Notifier interface {
	Publish(event models.ReqMessagePayload) error
	Close()
}

type EventPublisher struct {
	client   pulsar.Client
	producer pulsar.Producer
}

const maxRetries = 3 // Hardcoded and slightly random for now - can be made configurable

// Initializes the Pulsar client and producer
func NewEventPublisher(pulsarURL, topic string) (*EventPublisher, error) {
	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: pulsarURL,
	})
	if err != nil {
		return nil, fmt.Errorf("could not create Pulsar client: %w", err)
	}

	producer, err := client.CreateProducer(pulsar.ProducerOptions{
		Topic: topic,
	})
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("could not create Pulsar producer: %w", err)
	}

	log.Info().Msg("Pulsar client and producer initialized successfully")
	return &EventPublisher{
		client:   client,
		producer: producer,
	}, nil
}

// Tries to publish an event, retrying if necessary
func (p *EventPublisher) Publish(event models.ReqMessagePayload) error {
	message, err := json.Marshal(event)
	if err != nil {
		log.Error().Err(err).Msg("Failed to serialize event")
		return err
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := p.producer.Send(context.Background(), &pulsar.ProducerMessage{
			Payload: message,
		})
		if err == nil {
			return err
		}
		log.Error().Int("attempt", attempt).Err(err).Msg("Failed to send event to Pulsar")

		// Reconnect or retry logic, wait before retrying
		if attempt < maxRetries {
			time.Sleep(2 * time.Second) // Simple backoff, adjust as needed
		}
	}
	log.Error().Int("maxRetries", maxRetries).Msg("Giving up after maxRetries attempts")
	return fmt.Errorf("failed to publish event after %d attempts: %w", maxRetries, err)

}

// Close the Pulsar client, producer, and stop the goroutine
func (p *EventPublisher) Close() {
	p.producer.Close()
	p.client.Close()
	log.Info().Msg("Pulsar client and producer closed successfully")
}
