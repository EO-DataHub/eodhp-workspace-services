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

type AWSEFSAccessPoint struct {
	Name        string
	FSID        string
	RootDir     string
	UID         int
	GID         int
	Permissions string
}

type AWSS3Bucket struct {
	BucketName      string
	BucketPath      string
	AccessPointName string
	EnvVar          string
}

type Stores struct {
	Object []AWSS3Bucket
	Block  []AWSEFSAccessPoint
}

type MessagePayload struct {
	Status       string    `json:"status"`
	Name         string    `json:"name"`
	Account      uuid.UUID `json:"account"`
	AccountOwner string    `json:"accountOwner"`
	MemberGroup  string    `json:"memberGroup"`
	Timestamp    int64     `json:"timestamp"`
	Stores       Stores    `json:"stores"`
}

type AckPayload struct {
	WorkspaceID string `json:"workspace_id"`
	Status      string `json:"status"` // The status returned after processing (e.g., "created", "failed")
}

type Notifier interface {
	Publish(event MessagePayload) error
	ReceiveAck(workspaceID string) (*AckPayload, error)
	Close()
}

type EventPublisher struct {
	client    pulsar.Client
	producer  pulsar.Producer
	consumer  pulsar.Consumer
	eventChan chan MessagePayload // Queue our messages in this Channel
	quitChan  chan struct{}       // Channel to signal shutdown
}

const maxRetries = 3 // Hardcoded and slightly random for now - can be made configurable

// Initializes the Pulsar client and producer
func NewEventPublisher(pulsarURL, topic string, ackTopic string) (*EventPublisher, error) {
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

	// Set up a consumer to receive ACKs
	consumer, err := client.Subscribe(pulsar.ConsumerOptions{
		Topic:            ackTopic,
		SubscriptionName: "workspace-subscription",
		Type:             pulsar.Shared,
	})
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("could not create Pulsar consumer: %w", err)
	}

	// Initialize the EventPublisher
	publisher := &EventPublisher{
		client:    client,
		producer:  producer,
		consumer:  consumer,
		eventChan: make(chan MessagePayload, 100), // Buffered channel for events - should be ok for now
		quitChan:  make(chan struct{}),
	}

	// Start the goroutine that processes events - we want this as separate go routine to prevent blocking API calls
	go publisher.run()

	log.Println("Pulsar client and producer/consumer initialized successfully")
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
func (p *EventPublisher) publishWithRetry(event MessagePayload) {
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

// waits for an ACK related to the workspace creation
func (p *EventPublisher) ReceiveAck(workspaceID string) (*AckPayload, error) {
	ackChan := make(chan *AckPayload)
	timeout := time.After(30 * time.Second) // Timeout after 30 seconds

	// Listen for the ACK in a separate goroutine
	go func() {
		for {
			msg, err := p.consumer.Receive(context.Background())
			if err != nil {
				log.Println("Error receiving Pulsar message:", err)
				continue
			}

			fmt.Println("Received ACK:", string(msg.Payload()))

			var ack AckPayload
			err = json.Unmarshal(msg.Payload(), &ack)
			if err != nil {
				log.Println("Error unmarshalling ACK:", err)
				continue
			}

			ackChan <- &ack
			p.consumer.Ack(msg) // Acknowledge the message
			return

		}
	}()

	// Wait for ACK or timeout
	select {
	case ack := <-ackChan:
		return ack, nil
	case <-timeout:
		return nil, fmt.Errorf("timeout waiting for ACK")
	}
}

// Instead of publishing directly, it enqueues the event on the event channel
func (p *EventPublisher) Publish(event MessagePayload) error {
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
	p.consumer.Close()
	p.client.Close()
	close(p.eventChan)
	log.Println("Pulsar client and producer closed successfully")
}
