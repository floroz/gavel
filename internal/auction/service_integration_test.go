package auction_test

import (
	"context"
	"testing"
	"time"

	"github.com/floroz/auction-system/internal/auction"
	"github.com/floroz/auction-system/internal/infra/database"
	"github.com/floroz/auction-system/internal/testhelpers"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedTestItem inserts a test item into the database
func seedTestItem(t *testing.T, pool *pgxpool.Pool, item *auction.Item) {
	t.Helper()
	ctx := context.Background()
	query := `
		INSERT INTO items (id, title, description, start_price, current_highest_bid, end_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := pool.Exec(ctx, query,
		item.ID,
		item.Title,
		item.Description,
		item.StartPrice,
		item.CurrentHighestBid,
		item.EndAt,
		item.CreatedAt,
		item.UpdatedAt,
	)
	require.NoError(t, err, "Failed to seed test item")
}

// TestAuctionService_PlaceBid_Integration_Success tests the full PlaceBid flow with real database
func TestAuctionService_PlaceBid_Success(t *testing.T) {
	// Setup: Start PostgreSQL container and run migrations
	testDB := testhelpers.NewTestDatabase(t, "../../migrations")
	defer testDB.Close()

	pool := testDB.Pool

	// Create repositories
	txManager := database.NewPostgresTransactionManager(pool, 5*time.Second)
	bidRepo := database.NewPostgresBidRepository(pool)
	itemRepo := database.NewPostgresItemRepository(pool)
	outboxRepo := database.NewPostgresOutboxRepository(pool)

	// Create service
	service := auction.NewAuctionService(txManager, bidRepo, itemRepo, outboxRepo)

	// Seed test data: Create an auction item
	itemID := uuid.New()
	userID := uuid.New()
	testItem := &auction.Item{
		ID:                itemID,
		Title:             "Vintage Guitar",
		Description:       "A beautiful 1960s guitar",
		StartPrice:        100000, // $1000.00 in cents
		CurrentHighestBid: 0,
		EndAt:             time.Now().Add(24 * time.Hour), // Ends tomorrow
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	seedTestItem(t, pool, testItem)

	// Test: Place a valid bid
	ctx := context.Background()
	cmd := auction.PlaceBidCommand{
		ItemID: itemID,
		UserID: userID,
		Amount: 150000, // $1500.00 in cents
	}

	// Act
	bid, err := service.PlaceBid(ctx, cmd)

	// Assert: No error
	require.NoError(t, err, "PlaceBid should succeed")
	assert.NotNil(t, bid)
	assert.Equal(t, itemID, bid.ItemID)
	assert.Equal(t, userID, bid.UserID)
	assert.Equal(t, int64(150000), bid.Amount)

	// Verify: Bid was saved in database
	savedBid, err := bidRepo.GetBidByID(ctx, bid.ID)
	require.NoError(t, err, "Should be able to retrieve saved bid")
	assert.Equal(t, bid.ID, savedBid.ID)
	assert.Equal(t, bid.Amount, savedBid.Amount)

	// Verify: Item's highest bid was updated
	updatedItem, err := itemRepo.GetItemByID(ctx, itemID)
	require.NoError(t, err, "Should be able to retrieve updated item")
	assert.Equal(t, int64(150000), updatedItem.CurrentHighestBid, "Item's highest bid should be updated")

	// Verify: Outbox event was created
	tx, err := txManager.BeginTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)

	events, err := outboxRepo.GetPendingEvents(ctx, tx, 10)
	require.NoError(t, err, "Should be able to retrieve outbox events")
	require.Len(t, events, 1, "Should have exactly one outbox event")

	event := events[0]
	assert.Equal(t, auction.EventTypeBidPlaced, event.EventType)
	assert.Equal(t, auction.OutboxStatusPending, event.Status)
	assert.NotEmpty(t, event.Payload, "Event payload should not be empty")
}
