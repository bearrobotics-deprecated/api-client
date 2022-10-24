package amqp

import (
	"context"
	"fmt"
	"log"
	"time"

	amqpgo "github.com/rabbitmq/amqp091-go"
)

// Caller handles RPC service calls via AMQP protocol.
type Caller struct {
	ch *amqpgo.Channel
}

// NewCaller creates a new service caller. Service calls are done over provided
// channel to the message borker.
func NewCaller(ch *amqpgo.Channel) *Caller {
	return &Caller{ch: ch}
}

// Call invokes a service call (RPC) on the remote robot `robotID`.
// Both call args and return values are JSON encoded.
func (c *Caller) Call(robotID string, command []byte, timeout time.Duration) (res []byte, err error) {
	ch := c.ch
	ctx := context.Background()

	routingKey := robotID
	corrID := randomString(32)
	consumerID := randomString(32)

	q, err := ch.QueueDeclare(
		"",                  // name
		false,               // durable
		true,                // delete when unused
		true,                // exclusive
		false,               // noWait
		GetNewQueueParams(), // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare queue (passively): %w", err)
	}

	msgs, err := ch.Consume(
		q.Name,     // queue
		consumerID, // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	if err != nil {
		return nil, fmt.Errorf("failed to conumee queue: %w", err)
	}

	// Cancel the consumer when done.
	defer func() {
		if err != ch.Cancel(consumerID, false) {
			log.Printf("Failed to cancel subscriber: %v", err)
		}
	}()

	err = ch.PublishWithContext(
		ctx,        // context
		"",         // exchange
		routingKey, // routing key = robot id
		false,      // mandatory
		false,      // immediate
		amqpgo.Publishing{
			ContentType:   "text/json",
			CorrelationId: corrID,
			ReplyTo:       q.Name,
			Body:          command,
		})

	if err != nil {
		return nil, fmt.Errorf("failed to publish call command: %w", err)
	}

	for {
		select {
		case <-time.After(timeout):
			return nil, fmt.Errorf("timed out")
		case d, ok := <-msgs:
			if !ok {
				return nil, fmt.Errorf("channel is closed")
			}
			if corrID == d.CorrelationId {
				return d.Body, nil
			}
		}
	}
}
