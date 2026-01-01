package bids

import (
	"time"

	"github.com/google/uuid"
)

// Bid represents an auction bid
type Bid struct {
	ID        uuid.UUID `db:"id"`
	ItemID    uuid.UUID `db:"item_id"`
	UserID    uuid.UUID `db:"user_id"`
	Amount    int64     `db:"amount"`
	CreatedAt time.Time `db:"created_at"`
}
