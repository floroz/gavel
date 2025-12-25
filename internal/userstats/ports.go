package userstats

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserStatsRepository interface {
	// SaveUserStats updates the user stats (Upsert)
	SaveUserStats(ctx context.Context, tx pgx.Tx, userStats *UserStats) error

	// GetUserStats retrieves stats for a user
	GetUserStats(ctx context.Context, userID uuid.UUID) (*UserStats, error)

	// MarkEventProcessed marks an event as processed to prevent duplicates
	MarkEventProcessed(ctx context.Context, tx pgx.Tx, eventID uuid.UUID) error

	// IsEventProcessed checks if an event has already been processed
	IsEventProcessed(ctx context.Context, tx pgx.Tx, eventID uuid.UUID) (bool, error)
}
