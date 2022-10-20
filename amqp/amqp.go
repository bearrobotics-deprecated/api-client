// Package amqp provides abstraction for commaon AMQP091 functions
// including service call (RPC) and Pub/Sub.
package amqp

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	amqpgo "github.com/rabbitmq/amqp091-go"
)

// randInt returns a random integer with the give reange.
func randInt(min int, max int) int {
	return min + rand.Int()%(max-min)
}

// randomString returns a random string of lowercase ASCII characters with the given length.
func randomString(l int) string {
	bytes := make([]byte, l)
	for i := 0; i < l; i++ {
		bytes[i] = byte(randInt(97, 122))
	}
	return string(bytes)
}

// Delivery represents basic data received by subscribers. In addition, it
// annotates messages with their origin ID.
type Delivery struct {
	Origin string
	Body   []byte
}

// ChannelIntf is an interface type for amqpgo.Channel implementations.
type ChannelIntf interface {
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqpgo.Table) (<-chan amqpgo.Delivery, error)
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqpgo.Publishing) error
	ExchangeDeclarePassive(name, kind string, durable, autoDelete, internal, noWait bool, args amqpgo.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqpgo.Table) (amqpgo.Queue, error)
	QueueBind(name, key, exchange string, noWait bool, args amqpgo.Table) error
	ExchangeBind(destination, key, source string, noWait bool, args amqpgo.Table) error
	Qos(prefetchCount, prefetchSize int, global bool) error
}

// Caller handles RPC service calls via AMQP protocol.
type Caller struct {
	ch ChannelIntf
}

type topicConsumer func(ChannelIntf, string) (<-chan amqpgo.Delivery, error)

// Subscriber handles subscribing to published updates.
type Subscriber struct {
	ch      ChannelIntf
	cancel  context.CancelFunc
	consume topicConsumer
}

// NewCaller creates a new service caller. Service calls are done over provided
// channel to the message borker.
func NewCaller(ch ChannelIntf) *Caller {
	return &Caller{ch: ch}
}

// NewSubscriber creates a subscriber to messages published to the message broker.
// The Provided channel `ch` establishes connection to the message broker.
func NewSubscriber(ch ChannelIntf) *Subscriber {
	noOp := func() {}
	return &Subscriber{ch: ch, cancel: noOp, consume: consumeTopic}
}

// Dial returns a new Connection to rabbitMQ server given user credential and address.
//
// The connection will be over TCP and authentication will be plain using
// provided user/pass.
func Dial(user, pass, serverAddr, vHost string) (*amqpgo.Connection, error) {
	u := &url.URL{
		Scheme: "amqp",
		User:   url.UserPassword(user, pass),
		Host:   serverAddr,
		Path:   vHost,
	}
	return amqpgo.Dial(u.String())
}

// DialTLS returns a new Connection to rabbitMQ server given user credential and address.
//
// The connection will be secured with TLS and authenticated with  certificates.
// In TLS mode we expect ca_certificate.pem, certificate.pem, and key.pem
// files to be accessible from certPath. Since RabbitMQ server is configured to authenticate
// users with their certificate (see rabbitmq_auth_mechanism_ssl), username will be equal
// to certificate common name (CN).
func DialTLS(serverAddr, vHost, certPath string) (*amqpgo.Connection, error) {
	u := &url.URL{
		Scheme: "amqps",
		// User:  No user password allowed
		Host: serverAddr,
		Path: vHost,
	}
	cfg, err := getTLSConfig(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get dial config: %w", err)
	}

	return amqpgo.DialTLS_ExternalAuth(u.String(), cfg)
}

// Call invokes a service call (RPC) on the remote robot `robotID`.
// Both call args and return values are JSON encoded.
func (c *Caller) Call(robotID string, command []byte, timeout time.Duration) (res []byte, err error) {
	ch := c.ch
	ctx := context.Background()

	routingKey := robotID
	corrID := randomString(32)

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
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return nil, fmt.Errorf("failed to conumee queue: %w", err)
	}

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

// Subscribe returns a subscriber to the given topic. Topics are implemented with
// fanout exchange on AMQP message broker to which services publish various data.
func (s *Subscriber) Subscribe(id, topic string) (chan Delivery, error) {
	err := s.Unsubscribe()
	if err != nil {
		return nil, err
	}

	ch, err := s.consume(s.ch, topic)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	d := make(chan Delivery)
	// Convert amqpgo.Delivery to Delivery
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
	return d, nil
}

// Unsubscribe tries to unsubscribe from an existing subscription that is
// established by prior call to Subscribe() method.
func (s *Subscriber) Unsubscribe() error {
	// Cancel pending subscription function if any exists.
	// Otherwise do nothing.
	if s.cancel == nil {
		return fmt.Errorf("cancel method has nil value")
	}
	s.cancel()
	return nil
}

// consumeTopic returns a consumer to a fanout exchange with given name published by the service.
func consumeTopic(ch ChannelIntf, topic string) (d <-chan amqpgo.Delivery, err error) {
	err = ch.ExchangeDeclarePassive(
		topic,    // name
		"fanout", // type
		true,     // durable
		false,    // auto-deleted
		false,    // internal
		false,    // no-wait
		nil,      // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare exchange (passively): %w", err)
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
		return nil, fmt.Errorf("failed to declare a queue: %w", err)
	}

	err = ch.QueueBind(
		queue.Name, // queue name
		"",         // routing key
		topic,      // exchange
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to bind a queue: %w", err)
	}

	d, err = ch.Consume(
		queue.Name, // queue
		"",         // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to queue: %w", err)
	}

	return d, nil
}
