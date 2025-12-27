package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/floroz/gavel/services/bid-service/internal/domain/bids"
)

// PostgresBidRepository implements bids.BidRepository using pgx
type PostgresBidRepository struct {
	pool *pgxpool.Pool // Keep pool for read-only operations
}

// NewPostgresBidRepository creates a new PostgreSQL bid repository
func NewPostgresBidRepository(pool *pgxpool.Pool) *PostgresBidRepository {
	return &PostgresBidRepository{pool: pool}
}

// SaveBid saves a bid using the provided database connection (pool or transaction)
func (r *PostgresBidRepository) SaveBid(ctx context.Context, tx pgx.Tx, bid *bids.Bid) error {
	query := `
		INSERT INTO bids (id, item_id, user_id, amount, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := tx.Exec(ctx, query,
		bid.ID,
		bid.ItemID,
		bid.UserID,
		bid.Amount,
		bid.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert bid: %w", err)
	}
	return nil
}

// GetBidByID retrieves a bid by its ID
func (r *PostgresBidRepository) GetBidByID(ctx context.Context, bidID uuid.UUID) (*bids.Bid, error) {
	query := `
		SELECT id, item_id, user_id, amount, created_at
		FROM bids
		WHERE id = $1
	`
	var bid bids.Bid
	err := r.pool.QueryRow(ctx, query, bidID).Scan(
		&bid.ID,
		&bid.ItemID,
		&bid.UserID,
		&bid.Amount,
		&bid.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("bid not found")
		}
		return nil, fmt.Errorf("failed to get bid: %w", err)
	}
	return &bid, nil
}

// GetBidsByItemID retrieves all bids for an item
func (r *PostgresBidRepository) GetBidsByItemID(ctx context.Context, itemID uuid.UUID) ([]*bids.Bid, error) {
	query := `
		SELECT id, item_id, user_id, amount, created_at
		FROM bids
		WHERE item_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.pool.Query(ctx, query, itemID)
	if err != nil {
		return nil, fmt.Errorf("failed to query bids: %w", err)
	}
	defer rows.Close()

	var result []*bids.Bid
	for rows.Next() {
		var bid bids.Bid
		if err := rows.Scan(
			&bid.ID,
			&bid.ItemID,
			&bid.UserID,
			&bid.Amount,
			&bid.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan bid: %w", err)
		}
		result = append(result, &bid)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating bids: %w", err)
	}

	return result, nil
}
