package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/floroz/gavel/pkg/events"
	"github.com/floroz/gavel/pkg/testhelpers"
	"github.com/floroz/gavel/services/auth-service/internal/adapters/database"
)

func TestOutboxRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// 1. Setup Database
	// Path to migrations relative to this file
	migrationsPath := "../../../migrations"
	td := testhelpers.NewTestDatabase(t, migrationsPath)
	defer td.Close()

	repo := database.NewPostgresOutboxRepository(td.Pool)
	ctx := context.Background()

	t.Run("CreateEvent_Success", func(t *testing.T) {
		event := &events.OutboxEvent{
			ID:        uuid.New(),
			EventType: "user.created",
			Payload:   []byte(`{"foo":"bar"}`),
			Status:    events.OutboxStatusPending,
			CreatedAt: time.Now().UTC(),
		}

		// Start a transaction as required by the interface
		tx, err := td.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		err = repo.CreateEvent(ctx, tx, event)
		require.NoError(t, err)

		err = tx.Commit(ctx)
		require.NoError(t, err)

		// Verify it exists in DB
		var status string
		err = td.Pool.QueryRow(ctx, "SELECT status FROM outbox_events WHERE id = $1", event.ID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, string(events.OutboxStatusPending), status)
	})

	t.Run("UpdateEventStatus_Success", func(t *testing.T) {
		// Create an initial event
		event := &events.OutboxEvent{
			ID:        uuid.New(),
			EventType: "user.updated",
			Payload:   []byte(`{"foo":"baz"}`),
			Status:    events.OutboxStatusPending,
			CreatedAt: time.Now().UTC(),
		}

		tx, err := td.Pool.Begin(ctx)
		require.NoError(t, err)
		err = repo.CreateEvent(ctx, tx, event)
		require.NoError(t, err)
		err = tx.Commit(ctx)
		require.NoError(t, err)

		// Update status
		tx, err = td.Pool.Begin(ctx)
		require.NoError(t, err)
		defer tx.Rollback(ctx)

		err = repo.UpdateEventStatus(ctx, tx, event.ID, events.OutboxStatusPublished)
		require.NoError(t, err)

		err = tx.Commit(ctx)
		require.NoError(t, err)

		// Verify
		var status string
		var processedAt *time.Time
		err = td.Pool.QueryRow(ctx, "SELECT status, processed_at FROM outbox_events WHERE id = $1", event.ID).Scan(&status, &processedAt)
		require.NoError(t, err)
		assert.Equal(t, string(events.OutboxStatusPublished), status)
		assert.NotNil(t, processedAt)
	})
}
