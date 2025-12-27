package events

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"

	pkgdb "github.com/floroz/gavel/pkg/database"
	pkgevents "github.com/floroz/gavel/pkg/events"
	"github.com/floroz/gavel/services/bid-service/internal/adapters/database"
)

// BidEventsProducer orchestrates the process of relaying bid events from the outbox to RabbitMQ
type BidEventsProducer struct {
	relay     *OutboxRelay
	publisher *pkgevents.RabbitMQPublisher
}

// NewBidEventsProducer creates a new producer
func NewBidEventsProducer(pool *pgxpool.Pool, conn *amqp.Connection, logger *slog.Logger) (*BidEventsProducer, error) {
	publisher, err := pkgevents.NewRabbitMQPublisher(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to create publisher: %w", err)
	}

	txManager := pkgdb.NewPostgresTransactionManager(pool, 3*time.Second)
	outboxRepo := database.NewPostgresOutboxRepository(pool)

	relay := NewOutboxRelay(
		outboxRepo,
		publisher,
		txManager,
		10,                   // Batch size
		500*time.Millisecond, // Polling interval
		logger,
	)

	return &BidEventsProducer{
		relay:     relay,
		publisher: publisher,
	}, nil
}

// Run starts the relay loop
func (p *BidEventsProducer) Run(ctx context.Context) error {
	return p.relay.Run(ctx)
}

// Close closes the publisher channel
func (p *BidEventsProducer) Close() error {
	return p.publisher.Close()
}
