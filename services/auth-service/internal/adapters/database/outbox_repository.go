package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/floroz/gavel/services/auth-service/internal/domain/users"
)

// PostgresOutboxRepository implements users.OutboxRepository
type PostgresOutboxRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresOutboxRepository(pool *pgxpool.Pool) *PostgresOutboxRepository {
	return &PostgresOutboxRepository{pool: pool}
}

func (r *PostgresOutboxRepository) CreateEvent(ctx context.Context, tx pgx.Tx, event *users.OutboxEvent) error {
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
		return fmt.Errorf("failed to create outbox event: %w", err)
	}
	return nil
}

func (r *PostgresOutboxRepository) GetPendingEvents(ctx context.Context, tx pgx.Tx, limit int) ([]*users.OutboxEvent, error) {
	query := `
		SELECT id, event_type, payload, status, created_at, processed_at
		FROM outbox_events
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`
	rows, err := tx.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending events: %w", err)
	}
	defer rows.Close()

	var events []*users.OutboxEvent
	for rows.Next() {
		var event users.OutboxEvent
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

func (r *PostgresOutboxRepository) UpdateEventStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status users.OutboxStatus) error {
	query := `
		UPDATE outbox_events
		SET status = $1::outbox_status, processed_at = $2
		WHERE id = $3
	`
	now := time.Now()
	_, err := tx.Exec(ctx, query, status, now, id)
	if err != nil {
		return fmt.Errorf("failed to update event status: %w", err)
	}
	return nil
}
