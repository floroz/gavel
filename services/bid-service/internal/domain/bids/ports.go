package bids

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/floroz/gavel/services/bid-service/internal/domain/items"
)

// BidRepository defines the interface for bid persistence
type BidRepository interface {
	// SaveBid saves a bid within a transaction
	SaveBid(ctx context.Context, tx pgx.Tx, bid *Bid) error

	// GetBidByID retrieves a bid by its ID
	GetBidByID(ctx context.Context, bidID uuid.UUID) (*Bid, error)

	// GetBidsByItemID retrieves all bids for an item
	GetBidsByItemID(ctx context.Context, itemID uuid.UUID) ([]*Bid, error)
}

// OutboxRepository defines the interface for outbox event persistence
type OutboxRepository interface {
	// SaveEvent saves an outbox event within a transaction
	SaveEvent(ctx context.Context, tx pgx.Tx, event *OutboxEvent) error

	// GetPendingEvents retrieves pending events for processing
	// Uses SELECT FOR UPDATE SKIP LOCKED to prevent race conditions
	GetPendingEvents(ctx context.Context, tx pgx.Tx, limit int) ([]*OutboxEvent, error)

	// UpdateEventStatus updates the status of an event
	UpdateEventStatus(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, status OutboxStatus) error
}

// ItemRepository defines the interface for item persistence
type ItemRepository interface {
	// GetItemByID retrieves an item by its ID
	GetItemByID(ctx context.Context, itemID uuid.UUID) (*items.Item, error)

	// GetItemByIDForUpdate retrieves an item by its ID and locks it for update
	// This prevents race conditions when multiple users bid on the same item
	// Must be called within a transaction
	GetItemByIDForUpdate(ctx context.Context, tx pgx.Tx, itemID uuid.UUID) (*items.Item, error)

	// UpdateHighestBid updates the current highest bid for an item within a transaction
	UpdateHighestBid(ctx context.Context, tx pgx.Tx, itemID uuid.UUID, amount int64) error
}

// EventPublisher defines the interface for publishing events to a message broker
type EventPublisher interface {
	// Publish publishes a message to the broker
	Publish(ctx context.Context, exchange, routingKey string, body []byte) error
}
