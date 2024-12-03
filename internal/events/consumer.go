package events

import (
	"context"
	"fmt"

	"github.com/apache/pulsar-client-go/pulsar"
)

type EventConsumer struct {
	client   pulsar.Client
	consumer pulsar.Consumer
}

// NewEventConsumer initializes the Pulsar client and consumer.
func NewEventConsumer(pulsarURL, topic, subscription string) (*EventConsumer, error) {
	client, err := pulsar.NewClient(pulsar.ClientOptions{URL: pulsarURL})
	if err != nil {
		return nil, fmt.Errorf("could not create Pulsar client: %w", err)
	}

	consumer, err := client.Subscribe(pulsar.ConsumerOptions{
		Topic:            topic,
		SubscriptionName: subscription,
		Type:             pulsar.Shared,
		DLQ: &pulsar.DLQPolicy{
			MaxDeliveries:   3,
			DeadLetterTopic: topic + "-dlq",
		},
	})
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("could not create Pulsar consumer: %w", err)
	}

	return &EventConsumer{client: client, consumer: consumer}, nil
}

// ReceiveMessage retrieves a message from Pulsar.
func (c *EventConsumer) ReceiveMessage(ctx context.Context) (pulsar.Message, error) {
	msg, err := c.consumer.Receive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to receive message: %w", err)
	}
	return msg, nil
}

// Ack acknowledges a message.
func (c *EventConsumer) Ack(msg pulsar.Message) {
	c.consumer.Ack(msg)
}

// Nack negatively acknowledges a message.
func (c *EventConsumer) Nack(msg pulsar.Message) {
	c.consumer.Nack(msg)
}

// Close cleans up the Pulsar consumer and client.
func (c *EventConsumer) Close() {
	c.consumer.Close()
	c.client.Close()
}
