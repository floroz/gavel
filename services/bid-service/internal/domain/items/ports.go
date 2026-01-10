package items

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Repository defines the interface for item persistence
type Repository interface {
	// CreateItem creates a new auction item
	CreateItem(ctx context.Context, item *Item) error

	// GetItemByID retrieves an item by its ID
	GetItemByID(ctx context.Context, itemID uuid.UUID) (*Item, error)

	// GetItemByIDForUpdate retrieves an item by its ID and locks it for update
	// This prevents race conditions when multiple users bid on the same item
	// Must be called within a transaction
	GetItemByIDForUpdate(ctx context.Context, tx pgx.Tx, itemID uuid.UUID) (*Item, error)

	// UpdateItem updates an item's editable fields (title, description, images, category)
	UpdateItem(ctx context.Context, item *Item) error

	// UpdateStatus updates an item's status
	UpdateStatus(ctx context.Context, itemID uuid.UUID, status ItemStatus) error

	// UpdateHighestBid updates the current highest bid for an item within a transaction
	UpdateHighestBid(ctx context.Context, tx pgx.Tx, itemID uuid.UUID, amount int64) error

	// ListActiveItems retrieves active items with pagination
	ListActiveItems(ctx context.Context, limit, offset int) ([]*Item, error)

	// ListItemsBySellerID retrieves all items for a specific seller
	ListItemsBySellerID(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]*Item, error)

	// CountBidsByItemID returns the number of bids for a specific item
	CountBidsByItemID(ctx context.Context, itemID uuid.UUID) (int64, error)
}
