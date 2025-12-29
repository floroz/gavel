package events_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"

	"github.com/floroz/gavel/pkg/testhelpers"
	"github.com/floroz/gavel/services/bid-service/internal/adapters/events"
	"github.com/floroz/gavel/services/bid-service/internal/domain/bids"
)

func TestBidEventsProducerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// 1. Start RabbitMQ
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
	testDB := testhelpers.NewTestDatabase(t, "../../../migrations")
	defer testDB.Close()
	dbPool := testDB.Pool

	// 3. Setup RabbitMQ Connection
	conn, err := amqp.Dial(amqpURL)
	require.NoError(t, err)
	defer conn.Close()

	// 4. Setup Producer
	producer, err := events.NewBidEventsProducer(dbPool, conn, logger)
	require.NoError(t, err)
	defer producer.Close()

	// 5. Run Producer in Background
	ctxProducer, cancelProducer := context.WithCancel(ctx)
	errChan := make(chan error, 1)
	go func() {
		errChan <- producer.Run(ctxProducer)
	}()
	defer cancelProducer()

	// 6. Setup Test Consumer to verify messages
	consumerConn, err := amqp.Dial(amqpURL)
	require.NoError(t, err)
	defer consumerConn.Close()

	ch, err := consumerConn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	// Exchange should be declared by producer setup
	// Declare queue to bind
	q, err := ch.QueueDeclare("", false, false, true, false, nil)
	require.NoError(t, err)

	err = ch.QueueBind(q.Name, "bid.placed", "auction.events", false, nil)
	require.NoError(t, err)

	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	require.NoError(t, err)

	// 7. Insert Event into Outbox
	eventID := uuid.New()
	expectedPayload := []byte(`{"test":"producer_integration"}`)
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

	// 8. Verify Message Receipt
	select {
	case msg := <-msgs:
		assert.Equal(t, expectedPayload, msg.Body)
		assert.Equal(t, "bid.placed", msg.RoutingKey)
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message from RabbitMQ")
	}

	// 9. Verify DB Update
	require.Eventually(t, func() bool {
		var status string
		err = dbPool.QueryRow(ctx, "SELECT status FROM outbox_events WHERE id = $1", eventID).Scan(&status)
		if err != nil {
			return false
		}
		return status == string(bids.OutboxStatusPublished)
	}, 5*time.Second, 100*time.Millisecond, "Event status should be updated to 'published'")
}
