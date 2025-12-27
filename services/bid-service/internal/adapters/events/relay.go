package events

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/floroz/auction-system/pkg/database"
	"github.com/floroz/auction-system/services/bid-service/internal/domain/bids"
)

// OutboxRelay polls the database for pending events and publishes them
type OutboxRelay struct {
	outboxRepo bids.OutboxRepository
	publisher  bids.EventPublisher
	txManager  database.TransactionManager
	batchSize  int
	interval   time.Duration
	logger     *slog.Logger
}

// NewOutboxRelay creates a new outbox relay
func NewOutboxRelay(
	outboxRepo bids.OutboxRepository,
	publisher bids.EventPublisher,
	txManager database.TransactionManager,
	batchSize int,
	interval time.Duration,
	logger *slog.Logger,
) *OutboxRelay {
	return &OutboxRelay{
		outboxRepo: outboxRepo,
		publisher:  publisher,
		txManager:  txManager,
		batchSize:  batchSize,
		interval:   interval,
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
		// Exchange: auction.events
		// Routing Key: event_type (e.g., "bid.placed")
		err := r.publisher.Publish(ctx, "auction.events", string(event.EventType), event.Payload)
		if err != nil {
			// If publishing fails, we return error and the transaction rolls back.
			// The event remains 'pending' and will be retried.
			return fmt.Errorf("failed to publish event %s: %w", event.ID, err)
		}

		// Update status in DB
		err = r.outboxRepo.UpdateEventStatus(ctx, tx, event.ID, bids.OutboxStatusPublished)
		if err != nil {
			return fmt.Errorf("failed to update event status %s: %w", event.ID, err)
		}
	}

	return tx.Commit(ctx)
}
