package amqp

import amqpgo "github.com/rabbitmq/amqp091-go"

const (
	// MessageTTL is the time to deliver messages before they expire (milliseconds)
	MessageTTL = 5_000

	// MaxQueueLength is the maximum number of messages in a queue.
	MaxQueueLength = 10

	// MaxQueueSize is the maximum total size of the queue allowed in bytes.
	MaxQueueSize = 16 * 1024
)

// GetNewQueueParams returns parameters for declaring a queue on RabbitMQ server.
func GetNewQueueParams() amqpgo.Table {
	return amqpgo.Table{
		"x-message-ttl":      int32(MessageTTL),
		"x-max-length":       int32(MaxQueueLength),
		"x-max-length-bytes": int32(MaxQueueSize),
	}
}
