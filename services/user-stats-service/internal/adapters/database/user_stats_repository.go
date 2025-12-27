package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/floroz/gavel/services/user-stats-service/internal/domain/userstats"
)

type UserStatsRepository struct {
	pool *pgxpool.Pool
}

func NewUserStatsRepository(pool *pgxpool.Pool) *UserStatsRepository {
	return &UserStatsRepository{pool: pool}
}

// IncrementUserStats increments the user's bid stats atomically
func (r *UserStatsRepository) IncrementUserStats(ctx context.Context, tx pgx.Tx, userID uuid.UUID, amount int64, lastBidAt time.Time) error {
	query := `
		INSERT INTO user_stats (user_id, total_bids_placed, total_amount_bid, last_bid_at, created_at, updated_at)
		VALUES ($1, 1, $2, $3, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			total_bids_placed = user_stats.total_bids_placed + 1,
			total_amount_bid = user_stats.total_amount_bid + EXCLUDED.total_amount_bid,
			last_bid_at = EXCLUDED.last_bid_at,
			updated_at = NOW()
	`
	_, err := tx.Exec(ctx, query,
		userID,    // $1
		amount,    // $2
		lastBidAt, // $3
	)
	if err != nil {
		return fmt.Errorf("failed to increment user stats: %w", err)
	}
	return nil
}

func (r *UserStatsRepository) GetUserStats(ctx context.Context, userID uuid.UUID) (*userstats.UserStats, error) {
	query := `
		SELECT user_id, total_bids_placed, total_amount_bid, last_bid_at, created_at, updated_at
		FROM user_stats
		WHERE user_id = $1
	`
	var userStats userstats.UserStats
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&userStats.UserID,
		&userStats.TotalBidsPlaced,
		&userStats.TotalAmountBid,
		&userStats.LastBidAt,
		&userStats.CreatedAt,
		&userStats.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user stats not found")
		}
		return nil, fmt.Errorf("failed to get user stats: %w", err)
	}
	return &userStats, nil
}

func (r *UserStatsRepository) MarkEventProcessed(ctx context.Context, tx pgx.Tx, eventID uuid.UUID) error {
	query := `INSERT INTO processed_events (event_id) VALUES ($1)`
	_, err := tx.Exec(ctx, query, eventID)
	if err != nil {
		return fmt.Errorf("failed to mark event processed: %w", err)
	}
	return nil
}

func (r *UserStatsRepository) IsEventProcessed(ctx context.Context, tx pgx.Tx, eventID uuid.UUID) (bool, error) {
	query := `SELECT 1 FROM processed_events WHERE event_id = $1`
	var exists int
	err := tx.QueryRow(ctx, query, eventID).Scan(&exists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, fmt.Errorf("failed to check processed event: %w", err)
	}
	return true, nil
}
