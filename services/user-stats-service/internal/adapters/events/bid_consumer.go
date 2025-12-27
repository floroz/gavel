package events

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/protobuf/proto"

	pb "github.com/floroz/gavel/pkg/proto"
	"github.com/floroz/gavel/services/user-stats-service/internal/domain/userstats"
)

// BidConsumer consumes bid events and updates user statistics
type BidConsumer struct {
	conn    *amqp.Connection
	service *userstats.Service
	logger  *slog.Logger
}

// NewBidConsumer creates a new bid consumer
func NewBidConsumer(conn *amqp.Connection, service *userstats.Service, logger *slog.Logger) *BidConsumer {
	return &BidConsumer{
		conn:    conn,
		service: service,
		logger:  logger,
	}
}

// Run starts the consumer loop
func (c *BidConsumer) Run(ctx context.Context) error {
	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %w", err)
	}
	defer ch.Close()

	// Setup Exchange & Queue
	if setupErr := c.setupRabbitMQ(ch); setupErr != nil {
		return fmt.Errorf("failed to setup rabbitmq: %w", setupErr)
	}

	msgs, err := ch.Consume(
		"user_stats_bids", // queue
		"",                // consumer tag
		false,             // auto-ack
		false,             // exclusive
		false,             // no-local
		false,             // no-wait
		nil,               // args
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	c.logger.Info("Waiting for messages...")

	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-msgs:
			if !ok {
				return fmt.Errorf("channel closed")
			}
			c.logger.Info("Received message", "routing_key", d.RoutingKey)

			// Unmarshal Protobuf
			var event pb.BidPlaced
			if err := proto.Unmarshal(d.Body, &event); err != nil {
				c.logger.Error("Failed to unmarshal event", "error", err)
				// If we can't parse it, we probably can't process it ever.
				if nackErr := d.Nack(false, false); nackErr != nil {
					c.logger.Error("Failed to Nack message", "error", nackErr)
				}
				continue
			}

			// Map to Domain DTO
			bidEvent := userstats.BidPlacedEvent{
				EventID:   uuid.MustParse(event.BidId), // Using BidID as EventID as per main.go logic
				UserID:    uuid.MustParse(event.UserId),
				Amount:    event.Amount,
				Timestamp: event.Timestamp.AsTime(),
			}

			// Call Service (Idempotent)
			if err := c.service.ProcessBidPlaced(ctx, bidEvent); err != nil {
				c.logger.Error("Failed to process event", "error", err)
				// Nack(true) to requeue and retry
				if nackErr := d.Nack(false, true); nackErr != nil {
					c.logger.Error("Failed to Nack message (requeue)", "error", nackErr)
				}
			} else {
				// Ack on success
				if ackErr := d.Ack(false); ackErr != nil {
					c.logger.Error("Failed to Ack message", "error", ackErr)
				}
				c.logger.Info("Successfully processed event", "bid_id", event.BidId)
			}
		}
	}
}

func (c *BidConsumer) setupRabbitMQ(ch *amqp.Channel) error {
	err := ch.ExchangeDeclare(
		"auction.events", // name
		"topic",          // type
		true,             // durable
		false,            // auto-deleted
		false,            // internal
		false,            // no-wait
		nil,              // args
	)
	if err != nil {
		return err
	}

	q, err := ch.QueueDeclare(
		"user_stats_bids", // name
		true,              // durable
		false,             // delete when unused
		false,             // exclusive
		false,             // no-wait
		nil,               // args
	)
	if err != nil {
		return err
	}

	return ch.QueueBind(
		q.Name,           // queue name
		"bid.placed",     // routing key
		"auction.events", // exchange
		false,
		nil,
	)
}
