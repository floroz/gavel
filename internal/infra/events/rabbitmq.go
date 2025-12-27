package events

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQPublisher implements auction.EventPublisher
type RabbitMQPublisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
}

// NewRabbitMQPublisher creates a new RabbitMQ publisher
func NewRabbitMQPublisher(conn *amqp.Connection) (*RabbitMQPublisher, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Ensure the exchange exists
	err = ch.ExchangeDeclare(
		"auction.events", // name
		"topic",          // type
		true,             // durable
		false,            // auto-deleted
		false,            // internal
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		ch.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	return &RabbitMQPublisher{
		conn:    conn,
		channel: ch,
	}, nil
}

// Close closes the channel
func (p *RabbitMQPublisher) Close() error {
	return p.channel.Close()
}

// Publish publishes a message to the broker
func (p *RabbitMQPublisher) Publish(ctx context.Context, exchange, routingKey string, body []byte) error {
	return p.channel.PublishWithContext(ctx,
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType: "application/x-protobuf",
			Body:        body,
		},
	)
}
