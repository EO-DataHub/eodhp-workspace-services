package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/apache/pulsar-client-go/pulsar"
)

type EventPayload struct {
	WorkspaceID int    `json:"workspace_id"`
	Action      string `json:"action"` // create, update, delete
}

type EventPublisher struct {
	client   pulsar.Client
	producer pulsar.Producer
}

var publisher *EventPublisher

// Initializes the Pulsar client and producer
func InitEventPublisher(pulsarURL, topic string) error {
	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL: pulsarURL,
	})
	if err != nil {
		return fmt.Errorf("could not create Pulsar client: %w", err)
	}

	producer, err := client.CreateProducer(pulsar.ProducerOptions{
		Topic: topic,
	})
	if err != nil {
		client.Close()
		return fmt.Errorf("could not create Pulsar producer: %w", err)
	}

	// Set the global publisher
	publisher = &EventPublisher{
		client:   client,
		producer: producer,
	}

	log.Println("Pulsar client and producer initialized successfully")
	return nil
}

// PublishEvent publishes an event to Pulsar
func PublishEvent(workspaceID int, action string) error {
	if publisher == nil {
		return fmt.Errorf("event publisher is not initialized")
	}

	// Create event payload
	payload := EventPayload{
		WorkspaceID: workspaceID,
		Action:      action,
	}

	// Serialize the payload as JSON
	message, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("could not serialize event payload: %w", err)
	}

	// Publish the message to Pulsar
	_, err = publisher.producer.Send(context.Background(), &pulsar.ProducerMessage{
		Payload: message,
	})
	if err != nil {
		return fmt.Errorf("could not send event to Pulsar: %w", err)
	}

	log.Printf("Event sent to Pulsar: %s", message)
	return nil
}

// ClosePublisher closes the Pulsar client and producer
func ClosePublisher() {
	if publisher != nil {
		publisher.producer.Close()
		publisher.client.Close()
		log.Println("Pulsar client and producer closed successfully")
	}
}
