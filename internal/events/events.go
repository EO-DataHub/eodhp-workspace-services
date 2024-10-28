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
	client    pulsar.Client
	producer  pulsar.Producer
	eventChan chan models.ReqMessagePayload // Queue our messages in this Channel
	quitChan  chan struct{}                 // Channel to signal shutdown
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

	// Initialize the EventPublisher
	publisher := &EventPublisher{
		client:    client,
		producer:  producer,
		eventChan: make(chan models.ReqMessagePayload, 100), // Buffered channel for events - should be ok for now
		quitChan:  make(chan struct{}),
	}

	// Start the background workers
	go publisher.listenForEvents()
	log.Info().Msg("Pulsar client and producer initialized successfully")
	return publisher, nil
}

// Listens on the event channel and sends events to Pulsar
func (p *EventPublisher) listenForEvents() {
	for {
		select {
		case event := <-p.eventChan:
			p.publishWithRetry(event)
		case <-p.quitChan:
			log.Info().Msg("Stopping event publisher.")
			return
		}
	}
}

// Tries to publish an event, retrying if necessary
func (p *EventPublisher) publishWithRetry(event models.ReqMessagePayload) {
	message, err := json.Marshal(event)
	if err != nil {
		log.Error().Err(err).Msg("Failed to serialize event")
		return
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := p.producer.Send(context.Background(), &pulsar.ProducerMessage{
			Payload: message,
		})
		if err == nil {
			return
		}
		log.Error().Int("attempt", attempt).Err(err).Msg("Failed to send event to Pulsar")

		// Reconnect or retry logic, wait before retrying
		if attempt < maxRetries {
			time.Sleep(2 * time.Second) // Simple backoff, adjust as needed
		} else {
			log.Error().Int("maxRetries", maxRetries).Msg("Giving up after maxRetries attempts")
		}
	}
}

// Instead of publishing directly, it enqueues the event on the event channel
func (p *EventPublisher) Publish(event models.ReqMessagePayload) error {
	select {
	case p.eventChan <- event:
		log.Info().Interface("event", event).Msg("Event enqueued")
		return nil
	case <-time.After(2 * time.Second): // Timeout in case the channel is full - might need adjusting
		return fmt.Errorf("failed to enqueue event: queue is full")
	}
}

// Close the Pulsar client, producer, and stop the goroutine
func (p *EventPublisher) Close() {
	close(p.quitChan)
	p.producer.Close()
	p.client.Close()
	close(p.eventChan)
	log.Info().Msg("Pulsar client and producer closed successfully")
}
