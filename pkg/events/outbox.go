package events

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/floroz/gavel/pkg/database"
)

// OutboxStatus defines the status of an event in the outbox
type OutboxStatus string

const (
	OutboxStatusPending    OutboxStatus = "pending"
	OutboxStatusProcessing OutboxStatus = "processing"
	OutboxStatusPublished  OutboxStatus = "published"
	OutboxStatusFailed     OutboxStatus = "failed"
)

// OutboxEvent represents a generic event to be stored in the database
// Service-specific models can embed this or map to it
type OutboxEvent struct {
	ID          uuid.UUID    `db:"id"`
	EventType   string       `db:"event_type"`
	Payload     []byte       `db:"payload"`
	Status      OutboxStatus `db:"status"`
	CreatedAt   time.Time    `db:"created_at"`
	ProcessedAt *time.Time   `db:"processed_at"`
}

// OutboxRepository defines the interface for interacting with the outbox table
// Services must implement this interface or use a generic implementation
type OutboxRepository interface {
	GetPendingEvents(ctx context.Context, tx pgx.Tx, limit int) ([]*OutboxEvent, error)
	UpdateEventStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status OutboxStatus) error
}

// EventPublisher defines the interface for publishing events to a broker
type EventPublisher interface {
	Publish(ctx context.Context, exchange, routingKey string, body []byte) error
}

// OutboxRelay is a generic relay that polls the database for pending events and publishes them
type OutboxRelay struct {
	outboxRepo OutboxRepository
	publisher  EventPublisher
	txManager  database.TransactionManager
	batchSize  int
	interval   time.Duration
	exchange   string
	logger     *slog.Logger
}

// NewOutboxRelay creates a new generic outbox relay
func NewOutboxRelay(
	outboxRepo OutboxRepository,
	publisher EventPublisher,
	txManager database.TransactionManager,
	batchSize int,
	interval time.Duration,
	exchange string,
	logger *slog.Logger,
) *OutboxRelay {
	return &OutboxRelay{
		outboxRepo: outboxRepo,
		publisher:  publisher,
		txManager:  txManager,
		batchSize:  batchSize,
		interval:   interval,
		exchange:   exchange,
		logger:     logger,
	}
}

// Run starts the polling loop
func (r *OutboxRelay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	// Initial run
	if err := r.processBatch(ctx); err != nil {
		r.logger.Error("Error processing batch", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := r.processBatch(ctx); err != nil {
				r.logger.Error("Error processing batch", "error", err)
			}
		}
	}
}

func (r *OutboxRelay) processBatch(ctx context.Context) error {
	tx, err := r.txManager.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// Fetch pending events with FOR UPDATE SKIP LOCKED
	events, err := r.outboxRepo.GetPendingEvents(ctx, tx, r.batchSize)
	if err != nil {
		return fmt.Errorf("failed to fetch pending events: %w", err)
	}

	if len(events) == 0 {
		return nil // Nothing to do
	}

	r.logger.Info("Processing events", "count", len(events))

	for _, event := range events {
		// Publish to RabbitMQ
		// Exchange is configurable, Routing Key is the event type
		err := r.publisher.Publish(ctx, r.exchange, event.EventType, event.Payload)
		if err != nil {
			// If publishing fails, we return error and the transaction rolls back.
			// The event remains 'pending' and will be retried.
			return fmt.Errorf("failed to publish event %s: %w", event.ID, err)
		}

		// Update status in DB
		err = r.outboxRepo.UpdateEventStatus(ctx, tx, event.ID, OutboxStatusPublished)
		if err != nil {
			return fmt.Errorf("failed to update event status %s: %w", event.ID, err)
		}
	}

	return tx.Commit(ctx)
}
