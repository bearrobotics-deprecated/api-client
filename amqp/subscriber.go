package amqp

import (
	"context"
	"fmt"

	amqpgo "github.com/rabbitmq/amqp091-go"
)

type CancelFunc func() error

// Subscriber handles subscribing to published updates. It can only Subscribe
// to one exchange at a time.
type Subscriber struct {
	ch     *amqpgo.Channel
	cancel CancelFunc
}

func noOp() error {
	return nil
}

// NewSubscriber creates a subscriber to messages published to the message broker.
// The Provided channel `ch` establishes connection to the message broker.
func NewSubscriber(ch *amqpgo.Channel) *Subscriber {
	return &Subscriber{ch: ch, cancel: noOp}
}

// Subscribe returns a subscriber to the given topic. Topics are implemented with
// fanout exchange on AMQP message broker to which services publish various data.
func (s *Subscriber) Subscribe(id, topic string) (chan Delivery, error) {
	err := s.Unsubscribe()
	if err != nil {
		return nil, fmt.Errorf("failed to unsubscribe: %w", err)
	}

	ch, cancel, err := s.subscribe(id, topic)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	s.cancel = cancel
	return ch, nil
}

func (s *Subscriber) subscribe(id, topic string) (chan Delivery, CancelFunc, error) {
	ch, consumerID, err := s.consume(s.ch, topic)
	if err != nil {
		return nil, noOp, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	d := make(chan Delivery)
	// Convert amqpgo.Delivery to Delivery.
	go func() {
		defer close(d)
		for {
			select {
			case <-ctx.Done():
				return
			case val, ok := <-ch:
				if !ok {
					return
				}
				d <- Delivery{Origin: id, Body: val.Body}
			}
		}
	}()

	// This function will be triggered by s.Unsubscribe().
	shutdown := func() error {
		// Stops the consume loop and close Delivery channel d.
		cancel()
		// Cancels the consumer on RabbitMQ server.
		return s.cancelConsumer(consumerID)
	}
	return d, shutdown, nil
}

// Unsubscribe tries to unsubscribe from an existing subscription that is
// established by prior call to Subscribe() method.
func (s *Subscriber) Unsubscribe() error {
	// Cancel pending subscription function if any exists.
	// Otherwise do nothing.
	if s.cancel == nil {
		return fmt.Errorf("cancel method has nil value")
	}
	return s.cancel()
}

// consume returns a consumer to a fanout exchange with given name published by the service.
func (s *Subscriber) consume(ch *amqpgo.Channel, topic string) (<-chan amqpgo.Delivery, string, error) {
	if s.ch == nil {
		return nil, "", fmt.Errorf("cannot consumer from nil channel")
	}

	err := ch.ExchangeDeclarePassive(
		topic,    // name
		"fanout", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to declare exchange (passively): %w", err)
	}

	queue, err := ch.QueueDeclare(
		"",                  // name
		false,               // durable
		true,                // delete when unused
		true,                // exclusive
		false,               // no-wait
		GetNewQueueParams(), // arguments
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to declare a queue: %w", err)
	}

	err = ch.QueueBind(
		queue.Name, // queue name
		"",         // routing key
		topic,      // exchange
		false,
		nil,
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to bind a queue: %w", err)
	}

	consumerID := randomString(32)
	d, err := ch.Consume(
		queue.Name, // queue
		consumerID, // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to subscribe to queue: %w", err)
	}

	return d, consumerID, nil
}

func (s *Subscriber) cancelConsumer(id string) error {
	if s.ch == nil {
		return fmt.Errorf("cannot subscribe with nil channel")
	}
	return s.ch.Cancel(id, false)
}
