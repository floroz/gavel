package auction

import (
	"time"

	"github.com/google/uuid"
)

// Item represents an auction item
type Item struct {
	ID                uuid.UUID
	Title             string
	Description       string
	StartPrice        int64 // in cents/micros
	CurrentHighestBid int64
	EndAt             time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Bid represents a user's bid on an item
type Bid struct {
	ID        uuid.UUID
	ItemID    uuid.UUID
	UserID    uuid.UUID
	Amount    int64 // in cents/micros
	CreatedAt time.Time
}

// OutboxEvent represents an event waiting to be published
type OutboxEvent struct {
	ID          uuid.UUID
	EventType   EventType
	Payload     []byte // Serialized protobuf message
	Status      OutboxStatus
	CreatedAt   time.Time
	ProcessedAt *time.Time
}

// OutboxStatus represents the processing state of an outbox event
type OutboxStatus string

const (
	OutboxStatusPending    OutboxStatus = "pending"
	OutboxStatusProcessing OutboxStatus = "processing"
	OutboxStatusPublished  OutboxStatus = "published"
	OutboxStatusFailed     OutboxStatus = "failed"
)

// EventType represents the type of domain event
type EventType string

const (
	EventTypeBidPlaced EventType = "bid.placed"
	// Future events can be added here:
	// EventTypeBidRetracted EventType = "bid.retracted"
	// EventTypeAuctionEnded EventType = "auction.ended"
	// EventTypeItemCreated  EventType = "item.created"
)

// String returns the string representation of the event type
func (e EventType) String() string {
	return string(e)
}

// IsValid checks if the event type is valid
func (e EventType) IsValid() bool {
	switch e {
	case EventTypeBidPlaced:
		return true
	default:
		return false
	}
}

// PlaceBidCommand represents the command to place a bid
type PlaceBidCommand struct {
	ItemID uuid.UUID
	UserID uuid.UUID
	Amount int64
}
