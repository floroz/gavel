package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pkgevents "github.com/floroz/gavel/pkg/events"
)

// PostgresOutboxRepository implements bids.OutboxRepository using pgx
type PostgresOutboxRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresOutboxRepository creates a new PostgreSQL outbox repository
func NewPostgresOutboxRepository(pool *pgxpool.Pool) *PostgresOutboxRepository {
	return &PostgresOutboxRepository{pool: pool}
}

// SaveEvent saves an outbox event within a transaction
func (r *PostgresOutboxRepository) SaveEvent(ctx context.Context, tx pgx.Tx, event *pkgevents.OutboxEvent) error {
	query := `
		INSERT INTO outbox_events (id, event_type, payload, status, created_at)
		VALUES ($1, $2, $3, $4::outbox_status, $5)
	`
	_, err := tx.Exec(ctx, query,
		event.ID,
		event.EventType,
		event.Payload,
		event.Status,
		event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}
	return nil
}

// GetPendingEvents retrieves pending events for processing
// Uses SELECT FOR UPDATE SKIP LOCKED to prevent multiple workers from processing the same event
func (r *PostgresOutboxRepository) GetPendingEvents(ctx context.Context, tx pgx.Tx, limit int) ([]*pkgevents.OutboxEvent, error) {
	query := `
		SELECT id, event_type, payload, status, created_at, processed_at
		FROM outbox_events
		WHERE status = $1::outbox_status
		ORDER BY created_at ASC
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`

	rows, err := tx.Query(ctx, query, pkgevents.OutboxStatusPending, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending events: %w", err)
	}
	defer rows.Close()

	var events []*pkgevents.OutboxEvent
	for rows.Next() {
		var event pkgevents.OutboxEvent
		if err := rows.Scan(
			&event.ID,
			&event.EventType,
			&event.Payload,
			&event.Status,
			&event.CreatedAt,
			&event.ProcessedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}
		events = append(events, &event)
	}
	return events, nil
}

// UpdateEventStatus updates the status of an event
func (r *PostgresOutboxRepository) UpdateEventStatus(ctx context.Context, tx pgx.Tx, eventID uuid.UUID, status pkgevents.OutboxStatus) error {
	query := `
		UPDATE outbox_events
		SET status = $1::outbox_status, processed_at = $2
		WHERE id = $3
	`

	var processedAt *time.Time
	if status == pkgevents.OutboxStatusPublished || status == pkgevents.OutboxStatusFailed {
		now := time.Now()
		processedAt = &now
	}

	result, err := tx.Exec(ctx, query, status, processedAt, eventID)
	if err != nil {
		return fmt.Errorf("failed to update event status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("event not found")
	}

	return nil
}
