package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pkgdb "github.com/floroz/auction-system/pkg/database"
	"github.com/floroz/auction-system/services/bid-service/internal/domain/items"
)

// PostgresItemRepository implements bids.ItemRepository using pgx
type PostgresItemRepository struct {
	pool *pgxpool.Pool // Keep pool for non-transactional reads
}

// NewPostgresItemRepository creates a new PostgreSQL item repository
func NewPostgresItemRepository(pool *pgxpool.Pool) *PostgresItemRepository {
	return &PostgresItemRepository{pool: pool}
}

// GetItemByID retrieves an item by its ID (non-transactional read)
func (r *PostgresItemRepository) GetItemByID(ctx context.Context, itemID uuid.UUID) (*items.Item, error) {
	return r.getItemByID(ctx, r.pool, itemID, false)
}

// GetItemByIDForUpdate retrieves an item by its ID and locks it for update (transactional)
// This prevents race conditions when multiple users bid on the same item
func (r *PostgresItemRepository) GetItemByIDForUpdate(ctx context.Context, tx pgx.Tx, itemID uuid.UUID) (*items.Item, error) {
	return r.getItemByID(ctx, tx, itemID, true)
}

// getItemByID is the internal implementation that works with any DBTX
func (r *PostgresItemRepository) getItemByID(ctx context.Context, db pkgdb.DBTX, itemID uuid.UUID, forUpdate bool) (*items.Item, error) {
	query := `
		SELECT id, title, description, start_price, current_highest_bid, end_at, created_at, updated_at
		FROM items
		WHERE id = $1
	`
	if forUpdate {
		query += " FOR UPDATE"
	}

	var item items.Item
	err := db.QueryRow(ctx, query, itemID).Scan(
		&item.ID,
		&item.Title,
		&item.Description,
		&item.StartPrice,
		&item.CurrentHighestBid,
		&item.EndAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("item not found")
		}
		return nil, fmt.Errorf("failed to get item: %w", err)
	}
	return &item, nil
}

// UpdateHighestBid updates the current highest bid for an item within a transaction
func (r *PostgresItemRepository) UpdateHighestBid(ctx context.Context, tx pgx.Tx, itemID uuid.UUID, amount int64) error {
	query := `
		UPDATE items
		SET current_highest_bid = $1, updated_at = NOW()
		WHERE id = $2
	`
	result, err := tx.Exec(ctx, query, amount, itemID)
	if err != nil {
		return fmt.Errorf("failed to update highest bid: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("item not found")
	}

	return nil
}
