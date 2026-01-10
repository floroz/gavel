package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pkgdb "github.com/floroz/gavel/pkg/database"
	"github.com/floroz/gavel/services/bid-service/internal/domain/items"
)

// PostgresItemRepository implements bids.ItemRepository using pgx
type PostgresItemRepository struct {
	pool *pgxpool.Pool // Keep pool for non-transactional reads
}

// NewPostgresItemRepository creates a new PostgreSQL item repository
func NewPostgresItemRepository(pool *pgxpool.Pool) *PostgresItemRepository {
	return &PostgresItemRepository{pool: pool}
}

// CreateItem creates a new auction item
func (r *PostgresItemRepository) CreateItem(ctx context.Context, item *items.Item) error {
	query := `
		INSERT INTO items (id, title, description, start_price, current_highest_bid, end_at, created_at, updated_at, images, category, seller_id, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := r.pool.Exec(ctx, query,
		item.ID,
		item.Title,
		item.Description,
		item.StartPrice,
		item.CurrentHighestBid,
		item.EndAt,
		item.CreatedAt,
		item.UpdatedAt,
		item.Images,
		item.Category,
		item.SellerID,
		item.Status,
	)
	if err != nil {
		return fmt.Errorf("failed to create item: %w", err)
	}
	return nil
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
		SELECT id, title, description, start_price, current_highest_bid, end_at, created_at, updated_at, images, category, seller_id, status
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
		&item.Images,
		&item.Category,
		&item.SellerID,
		&item.Status,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("item not found")
		}
		return nil, fmt.Errorf("failed to get item: %w", err)
	}
	return &item, nil
}

// UpdateItem updates an item's editable fields
func (r *PostgresItemRepository) UpdateItem(ctx context.Context, item *items.Item) error {
	query := `
		UPDATE items
		SET title = $1, description = $2, images = $3, category = $4, updated_at = $5
		WHERE id = $6
	`
	result, err := r.pool.Exec(ctx, query,
		item.Title,
		item.Description,
		item.Images,
		item.Category,
		item.UpdatedAt,
		item.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("item not found")
	}

	return nil
}

// UpdateStatus updates an item's status
func (r *PostgresItemRepository) UpdateStatus(ctx context.Context, itemID uuid.UUID, status items.ItemStatus) error {
	query := `
		UPDATE items
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`
	result, err := r.pool.Exec(ctx, query, status, itemID)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("item not found")
	}

	return nil
}

// ListActiveItems retrieves active items with pagination
func (r *PostgresItemRepository) ListActiveItems(ctx context.Context, limit, offset int) ([]*items.Item, error) {
	query := `
		SELECT id, title, description, start_price, current_highest_bid, end_at, created_at, updated_at, images, category, seller_id, status
		FROM items
		WHERE status = $1 AND end_at > NOW()
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, items.ItemStatusActive, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list active items: %w", err)
	}
	defer rows.Close()

	var result []*items.Item
	for rows.Next() {
		var item items.Item
		err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.Description,
			&item.StartPrice,
			&item.CurrentHighestBid,
			&item.EndAt,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.Images,
			&item.Category,
			&item.SellerID,
			&item.Status,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		result = append(result, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return result, nil
}

// ListItemsBySellerID retrieves all items for a specific seller
func (r *PostgresItemRepository) ListItemsBySellerID(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]*items.Item, error) {
	query := `
		SELECT id, title, description, start_price, current_highest_bid, end_at, created_at, updated_at, images, category, seller_id, status
		FROM items
		WHERE seller_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, sellerID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list seller items: %w", err)
	}
	defer rows.Close()

	var result []*items.Item
	for rows.Next() {
		var item items.Item
		err := rows.Scan(
			&item.ID,
			&item.Title,
			&item.Description,
			&item.StartPrice,
			&item.CurrentHighestBid,
			&item.EndAt,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.Images,
			&item.Category,
			&item.SellerID,
			&item.Status,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan item: %w", err)
		}
		result = append(result, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return result, nil
}

// CountBidsByItemID returns the number of bids for a specific item
func (r *PostgresItemRepository) CountBidsByItemID(ctx context.Context, itemID uuid.UUID) (int64, error) {
	query := `SELECT COUNT(*) FROM bids WHERE item_id = $1`
	var count int64
	err := r.pool.QueryRow(ctx, query, itemID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count bids: %w", err)
	}
	return count, nil
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
