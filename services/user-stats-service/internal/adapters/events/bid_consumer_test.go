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
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/floroz/gavel/pkg/database"
	pb "github.com/floroz/gavel/pkg/proto"
	"github.com/floroz/gavel/pkg/testhelpers"
	infradb "github.com/floroz/gavel/services/user-stats-service/internal/adapters/database"
	"github.com/floroz/gavel/services/user-stats-service/internal/adapters/events"
	"github.com/floroz/gavel/services/user-stats-service/internal/domain/userstats"
)

func TestBidConsumerIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

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
	testDB := testhelpers.NewTestDatabase(t, "../../../migrations")
	defer testDB.Close()
	dbPool := testDB.Pool

	// 3. Setup Dependencies
	txManager := database.NewPostgresTransactionManager(dbPool, time.Second)
	statsRepo := infradb.NewUserStatsRepository(dbPool)
	statsService := userstats.NewService(statsRepo, txManager)

	// 4. Setup Consumer
	conn, err := amqp.Dial(amqpURL)
	require.NoError(t, err)
	defer conn.Close()

	consumer := events.NewBidConsumer(conn, statsService, logger)

	// 5. Run Consumer in Background
	ctxConsumer, cancelConsumer := context.WithCancel(ctx)
	errChan := make(chan error, 1)
	go func() {
		errChan <- consumer.Run(ctxConsumer)
	}()
	defer cancelConsumer()

	// Wait for consumer to be ready (simplest way is short sleep, or retry publish)
	time.Sleep(1 * time.Second)

	// 6. Publish Event
	publishConn, err := amqp.Dial(amqpURL)
	require.NoError(t, err)
	defer publishConn.Close()

	ch, err := publishConn.Channel()
	require.NoError(t, err)
	defer ch.Close()

	// Ensure exchange exists (consumer should have created it, but to be safe/explicit in test setup)
	// We rely on consumer to create it though, so this verifies consumer setup worked too.

	bidID := uuid.New()
	userID := uuid.New()
	amount := int64(100)

	event := &pb.BidPlaced{
		BidId:     bidID.String(),
		UserId:    userID.String(),
		ItemId:    uuid.New().String(),
		Amount:    amount,
		Timestamp: timestamppb.Now(),
	}
	body, err := proto.Marshal(event)
	require.NoError(t, err)

	err = ch.PublishWithContext(ctx,
		"auction.events", // exchange
		"bid.placed",     // routing key
		false,            // mandatory
		false,            // immediate
		amqp.Publishing{
			ContentType: "application/x-protobuf",
			Body:        body,
		},
	)
	require.NoError(t, err)

	// 7. Verify DB Update
	require.Eventually(t, func() bool {
		var totalAmount int64
		var totalBids int
		scanErr := dbPool.QueryRow(ctx, "SELECT total_amount_bid, total_bids_placed FROM user_stats WHERE user_id = $1", userID).Scan(&totalAmount, &totalBids)
		if scanErr != nil {
			return false
		}
		return totalAmount == amount && totalBids == 1
	}, 5*time.Second, 100*time.Millisecond, "User stats should be updated")

	// 8. Verify Idempotency (Publish same event again)
	err = ch.PublishWithContext(ctx,
		"auction.events",
		"bid.placed",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/x-protobuf",
			Body:        body,
		},
	)
	require.NoError(t, err)

	// Stats should NOT change
	time.Sleep(1 * time.Second) // Give it time to process if it was going to (bad check, but safe given Eventually above passed)

	var totalAmount int64
	var totalBids int
	err = dbPool.QueryRow(ctx, "SELECT total_amount_bid, total_bids_placed FROM user_stats WHERE user_id = $1", userID).Scan(&totalAmount, &totalBids)
	require.NoError(t, err)
	assert.Equal(t, amount, totalAmount)
	assert.Equal(t, 1, totalBids)
}
