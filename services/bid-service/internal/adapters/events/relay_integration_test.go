//go:build integration

package events_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"

	pkgdb "github.com/floroz/auction-system/pkg/database"
	pkgevents "github.com/floroz/auction-system/pkg/events"
	"github.com/floroz/auction-system/pkg/testhelpers"
	"github.com/floroz/auction-system/services/bid-service/internal/adapters/database"
	"github.com/floroz/auction-system/services/bid-service/internal/adapters/events"
	"github.com/floroz/auction-system/services/bid-service/internal/domain/bids"
)

// TestRelayIntegrationWithRabbitMQ runs a full integration test with a real RabbitMQ container
func TestRelayIntegrationWithRabbitMQ(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// 1. Start RabbitMQ Container
	rabbitmqContainer, err := rabbitmq.Run(ctx,
		"rabbitmq:3.12-management-alpine",
		rabbitmq.WithAdminPassword("password"),
	)
	require.NoError(t, err)
	defer func() {
		if termErr := rabbitmqContainer.Terminate(ctx); termErr != nil {
			t.Fatalf("failed to terminate container: %s", termErr)
		}
	}()

	amqpURL, err := rabbitmqContainer.AmqpURL(ctx)
	require.NoError(t, err)

	// 2. Setup Postgres
	// Path to migrations directory relative to this test file
	testDB := testhelpers.NewTestDatabase(t, "../../../migrations")
	defer testDB.Close()
	dbPool := testDB.Pool

	// 3. Setup Relay Components
	pubConn, err := amqp091.Dial(amqpURL)
	require.NoError(t, err)
	defer pubConn.Close()

	rabbitPublisher, err := pkgevents.NewRabbitMQPublisher(pubConn)
	require.NoError(t, err)
	defer rabbitPublisher.Close()

	txManager := pkgdb.NewPostgresTransactionManager(dbPool, time.Second)
	outboxRepo := database.NewPostgresOutboxRepository(dbPool)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	relay := events.NewOutboxRelay(
		outboxRepo,
		rabbitPublisher,
		txManager,
		10,
		50*time.Millisecond,
		logger,
	)

	// 4. Create a separate RabbitMQ consumer to verify message delivery
	conn, err := amqp091.Dial(amqpURL)
	require.NoError(t, err)
	defer conn.Close()

	ch, err := conn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	// Ensure queue matches what the publisher expects (or bind a queue to the exchange)
	err = ch.ExchangeDeclare("auction.events", "topic", true, false, false, false, nil)
	require.NoError(t, err)

	q, err := ch.QueueDeclare("", false, false, true, false, nil)
	require.NoError(t, err)

	err = ch.QueueBind(q.Name, "bid.placed", "auction.events", false, nil)
	require.NoError(t, err)

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	require.NoError(t, err)

	// 5. Seed Data
	eventID := uuid.New()
	expectedPayload := []byte(`{"test":"integration"}`)
	query := `
		INSERT INTO outbox_events (id, event_type, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err = dbPool.Exec(ctx, query,
		eventID,
		bids.EventTypeBidPlaced,
		expectedPayload,
		bids.OutboxStatusPending,
		time.Now(),
	)
	require.NoError(t, err)

	// 6. Run Relay
	ctxRelay, cancelRelay := context.WithCancel(ctx)
	go func() {
		_ = relay.Run(ctxRelay)
	}()
	defer cancelRelay()

	// 7. Verify Message Receipt in RabbitMQ
	select {
	case msg := <-msgs:
		assert.Equal(t, expectedPayload, msg.Body)
		assert.Equal(t, "bid.placed", msg.RoutingKey)
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for message from RabbitMQ")
	}

	// 8. Verify DB Update
	// Wait a moment for the DB update to commit
	require.Eventually(t, func() bool {
		var status string
		err = dbPool.QueryRow(ctx, "SELECT status FROM outbox_events WHERE id = $1", eventID).Scan(&status)
		if err != nil {
			return false
		}
		return status == string(bids.OutboxStatusPublished)
	}, 2*time.Second, 100*time.Millisecond, "Event status should be updated to 'published'")
}
