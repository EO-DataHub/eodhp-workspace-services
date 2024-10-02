package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/google/uuid"
)

type Notifier interface {
	Notify(event EventPayload) error
	Close()
}

type EventPayload struct {
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Action      string    `json:"action"` // e.g., create, update, delete - currently only create is used
}

type EventPublisher struct {
	client    pulsar.Client
	producer  pulsar.Producer
	eventChan chan EventPayload // Queue our messages in this Channel
	quitChan  chan struct{}     // Channel to signal shutdown
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
		eventChan: make(chan EventPayload, 100), // Buffered channel for events - should be ok for now
		quitChan:  make(chan struct{}),
	}

	// Start the goroutine that processes events - we want this as separate go routine to prevent blocking API calls
	go publisher.run()

	log.Println("Pulsar client and producer initialized successfully")
	return publisher, nil
}

// run listens on the event channel and sends events to Pulsar
func (p *EventPublisher) run() {
	for {
		select {
		case event := <-p.eventChan:
			p.publishWithRetry(event)
		case <-p.quitChan:
			log.Println("Stopping event publisher...")
			return
		}
	}
}

// Tries to publish an event, retrying if necessary
func (p *EventPublisher) publishWithRetry(event EventPayload) {
	message, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to serialize event: %v", err)
		return
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := p.producer.Send(context.Background(), &pulsar.ProducerMessage{
			Payload: message,
		})
		if err == nil {
			log.Printf("Event sent to Pulsar: %s", message)
			return
		}

		log.Printf("Failed to send event to Pulsar (attempt %d): %v", attempt, err)

		// Reconnect or retry logic, wait before retrying
		if attempt < maxRetries {
			time.Sleep(2 * time.Second) // Simple backoff, adjust as needed
		} else {
			log.Printf("Giving up after %d attempts", maxRetries)
		}
	}
}

// Instead of publishing directly, it enqueues the event on the event channel
func (p *EventPublisher) Notify(event EventPayload) error {
	select {
	case p.eventChan <- event:
		log.Printf("Event enqueued: %+v", event)
		return nil
	case <-time.After(2 * time.Second): // Timeout in case the channel is full - adjust as needed
		return fmt.Errorf("failed to enqueue event: queue is full")
	}
}

// Close the Pulsar client, producer, and stop the goroutine
func (p *EventPublisher) Close() {
	close(p.quitChan)
	p.producer.Close()
	p.client.Close()
	close(p.eventChan)
	log.Println("Pulsar client and producer closed successfully")
}
