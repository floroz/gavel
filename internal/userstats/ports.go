package userstats

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserStatsRepository interface {
	// IncrementUserStats increments the bid count and total amount for a user (Upsert)
	IncrementUserStats(ctx context.Context, tx pgx.Tx, userID uuid.UUID, amount int64, lastBidAt time.Time) error

	// GetUserStats retrieves stats for a user
	GetUserStats(ctx context.Context, userID uuid.UUID) (*UserStats, error)

	// MarkEventProcessed marks an event as processed to prevent duplicates
	MarkEventProcessed(ctx context.Context, tx pgx.Tx, eventID uuid.UUID) error

	// IsEventProcessed checks if an event has already been processed
	IsEventProcessed(ctx context.Context, tx pgx.Tx, eventID uuid.UUID) (bool, error)
}
