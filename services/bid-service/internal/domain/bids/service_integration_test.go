//go:build integration

package bids_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/floroz/gavel/pkg/database"
	"github.com/floroz/gavel/pkg/testhelpers"
	infradb "github.com/floroz/gavel/services/bid-service/internal/adapters/database"
	"github.com/floroz/gavel/services/bid-service/internal/domain/bids"
	"github.com/floroz/gavel/services/bid-service/internal/domain/items"
)

// seedTestItem inserts a test item into the database
func seedTestItem(t *testing.T, pool *pgxpool.Pool, item *items.Item) {
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

// testServices holds all service dependencies for testing
type testServices struct {
	Service    *bids.AuctionService
	TxManager  database.TransactionManager
	BidRepo    bids.BidRepository
	ItemRepo   bids.ItemRepository
	OutboxRepo bids.OutboxRepository
}

// setupAuctionService creates a service with all its dependencies for testing
func setupAuctionService(pool *pgxpool.Pool) *testServices {
	txManager := database.NewPostgresTransactionManager(pool, 5*time.Second)
	bidRepo := infradb.NewPostgresBidRepository(pool)
	itemRepo := infradb.NewPostgresItemRepository(pool)
	outboxRepo := infradb.NewPostgresOutboxRepository(pool)
	service := bids.NewAuctionService(txManager, bidRepo, itemRepo, outboxRepo)

	return &testServices{
		Service:    service,
		TxManager:  txManager,
		BidRepo:    bidRepo,
		ItemRepo:   itemRepo,
		OutboxRepo: outboxRepo,
	}
}

// TestAuctionService_PlaceBid_Integration_Success tests the full PlaceBid flow with real database
func TestAuctionService_PlaceBid_Success(t *testing.T) {
	// Setup: Start PostgreSQL container and run migrations
	testDB := testhelpers.NewTestDatabase(t, "../../../migrations")
	defer testDB.Close()

	pool := testDB.Pool
	svc := setupAuctionService(pool)

	// Seed test data: Create an auction item
	itemID := uuid.New()
	userID := uuid.New()
	testItem := &items.Item{
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
	cmd := bids.PlaceBidCommand{
		ItemID: itemID,
		UserID: userID,
		Amount: 150000, // $1500.00 in cents
	}

	// Act
	bid, err := svc.Service.PlaceBid(ctx, cmd)

	// Assert: No error
	require.NoError(t, err, "PlaceBid should succeed")
	assert.NotNil(t, bid)
	assert.Equal(t, itemID, bid.ItemID)
	assert.Equal(t, userID, bid.UserID)
	assert.Equal(t, int64(150000), bid.Amount)

	// Verify: Bid was saved in database
	savedBid, err := svc.BidRepo.GetBidByID(ctx, bid.ID)
	require.NoError(t, err, "Should be able to retrieve saved bid")
	assert.Equal(t, bid.ID, savedBid.ID)
	assert.Equal(t, bid.Amount, savedBid.Amount)

	// Verify: Item's highest bid was updated
	updatedItem, err := svc.ItemRepo.GetItemByID(ctx, itemID)
	require.NoError(t, err, "Should be able to retrieve updated item")
	assert.Equal(t, int64(150000), updatedItem.CurrentHighestBid, "Item's highest bid should be updated")

	// Verify: Outbox event was created
	tx, err := svc.TxManager.BeginTx(ctx)
	require.NoError(t, err)
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	events, err := svc.OutboxRepo.GetPendingEvents(ctx, tx, 10)
	require.NoError(t, err, "Should be able to retrieve outbox events")
	require.Len(t, events, 1, "Should have exactly one outbox event")

	event := events[0]
	assert.Equal(t, bids.EventTypeBidPlaced, event.EventType)
	assert.Equal(t, bids.OutboxStatusPending, event.Status)
	assert.NotEmpty(t, event.Payload, "Event payload should not be empty")
}

// TestAuctionService_PlaceBid_ItemNotFound tests bid on non-existent item
func TestAuctionService_PlaceBid_ItemNotFound(t *testing.T) {
	// Setup
	testDB := testhelpers.NewTestDatabase(t, "../../../migrations")
	defer testDB.Close()

	pool := testDB.Pool
	svc := setupAuctionService(pool)

	// No item seeded - item doesn't exist
	ctx := context.Background()
	cmd := bids.PlaceBidCommand{
		ItemID: uuid.New(), // Non-existent item
		UserID: uuid.New(),
		Amount: 150000,
	}

	// Act
	bid, err := svc.Service.PlaceBid(ctx, cmd)

	// Assert
	require.Error(t, err)
	assert.Nil(t, bid)
	assert.Contains(t, err.Error(), "item not found")
}

// TestAuctionService_PlaceBid_BidTooLow tests business rule validation
func TestAuctionService_PlaceBid_BidTooLow(t *testing.T) {
	// Setup
	testDB := testhelpers.NewTestDatabase(t, "../../../migrations")
	defer testDB.Close()

	pool := testDB.Pool
	svc := setupAuctionService(pool)

	// Seed item with existing high bid
	itemID := uuid.New()
	testItem := &items.Item{
		ID:                itemID,
		Title:             "Expensive Watch",
		Description:       "Luxury timepiece",
		StartPrice:        50000,
		CurrentHighestBid: 100000, // Current bid: $1000
		EndAt:             time.Now().Add(24 * time.Hour),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	seedTestItem(t, pool, testItem)

	// Try to place a lower bid
	ctx := context.Background()
	cmd := bids.PlaceBidCommand{
		ItemID: itemID,
		UserID: uuid.New(),
		Amount: 90000, // $900 - lower than current bid
	}

	// Act
	bid, err := svc.Service.PlaceBid(ctx, cmd)

	// Assert
	require.Error(t, err)
	assert.Nil(t, bid)
	assert.ErrorIs(t, err, bids.ErrBidTooLow)

	// Verify: No bid was saved
	bids, err := svc.BidRepo.GetBidsByItemID(ctx, itemID)
	require.NoError(t, err)
	assert.Empty(t, bids, "No bid should be saved when validation fails")

	// Verify: Item's highest bid unchanged
	item, err := svc.ItemRepo.GetItemByID(ctx, itemID)
	require.NoError(t, err)
	assert.Equal(t, int64(100000), item.CurrentHighestBid, "Highest bid should remain unchanged")
}

// TestAuctionService_PlaceBid_BidEqualToCurrent tests bid equal to current highest
func TestAuctionService_PlaceBid_BidEqualToCurrent(t *testing.T) {
	// Setup
	testDB := testhelpers.NewTestDatabase(t, "../../../migrations")
	defer testDB.Close()

	pool := testDB.Pool
	svc := setupAuctionService(pool)

	const bidAmount = 100000

	// Seed item
	itemID := uuid.New()
	testItem := &items.Item{
		ID:                itemID,
		Title:             "Rare Coin",
		StartPrice:        50000,
		CurrentHighestBid: bidAmount,
		EndAt:             time.Now().Add(24 * time.Hour),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	seedTestItem(t, pool, testItem)

	// Try to place equal bid
	ctx := context.Background()
	cmd := bids.PlaceBidCommand{
		ItemID: itemID,
		UserID: uuid.New(),
		Amount: bidAmount,
	}

	// Act
	bid, err := svc.Service.PlaceBid(ctx, cmd)

	// Assert
	require.Error(t, err)
	assert.Nil(t, bid)
	assert.ErrorIs(t, err, bids.ErrBidTooLow)
}

// TestAuctionService_PlaceBid_AuctionEnded tests business rule validation
func TestAuctionService_PlaceBid_AuctionEnded(t *testing.T) {
	// Setup
	testDB := testhelpers.NewTestDatabase(t, "../../../migrations")
	defer testDB.Close()

	pool := testDB.Pool
	svc := setupAuctionService(pool)

	// Seed item that already ended
	itemID := uuid.New()
	testItem := &items.Item{
		ID:                itemID,
		Title:             "Expired Auction",
		Description:       "This auction has ended",
		StartPrice:        50000,
		CurrentHighestBid: 75000,
		EndAt:             time.Now().Add(-1 * time.Hour), // Ended 1 hour ago
		CreatedAt:         time.Now().Add(-48 * time.Hour),
		UpdatedAt:         time.Now(),
	}
	seedTestItem(t, pool, testItem)

	// Try to place a bid
	ctx := context.Background()
	cmd := bids.PlaceBidCommand{
		ItemID: itemID,
		UserID: uuid.New(),
		Amount: 100000, // Higher than current bid
	}

	// Act
	bid, err := svc.Service.PlaceBid(ctx, cmd)

	// Assert
	require.Error(t, err)
	assert.Nil(t, bid)
	assert.ErrorIs(t, err, bids.ErrAuctionEnded)

	// Verify: No bid was saved
	bids, err := svc.BidRepo.GetBidsByItemID(ctx, itemID)
	require.NoError(t, err)
	assert.Empty(t, bids, "No bid should be saved when auction has ended")

	// Verify: Item's highest bid unchanged
	item, err := svc.ItemRepo.GetItemByID(ctx, itemID)
	require.NoError(t, err)
	assert.Equal(t, int64(75000), item.CurrentHighestBid)
}

// TestAuctionService_PlaceBid_RejectLowerBid tests that transaction rolls back on validation error
func TestAuctionService_PlaceBid_RejectLowerBid(t *testing.T) {
	// Setup
	testDB := testhelpers.NewTestDatabase(t, "../../../migrations")
	defer testDB.Close()

	pool := testDB.Pool
	svc := setupAuctionService(pool)

	// Seed item
	itemID := uuid.New()
	testItem := &items.Item{
		ID:                itemID,
		Title:             "Test Item",
		StartPrice:        50000,
		CurrentHighestBid: 0,
		EndAt:             time.Now().Add(24 * time.Hour),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	seedTestItem(t, pool, testItem)

	ctx := context.Background()

	// Place first bid successfully
	cmd1 := bids.PlaceBidCommand{
		ItemID: itemID,
		UserID: uuid.New(),
		Amount: 100000,
	}
	bid1, err := svc.Service.PlaceBid(ctx, cmd1)
	require.NoError(t, err)
	require.NotNil(t, bid1)

	// Try to place a lower bid (should fail validation)
	cmd2 := bids.PlaceBidCommand{
		ItemID: itemID,
		UserID: uuid.New(),
		Amount: 90000, // Too low
	}
	_, err = svc.Service.PlaceBid(ctx, cmd2)
	require.Error(t, err)

	// Verify: Only one bid exists (the first one)
	bids, err := svc.BidRepo.GetBidsByItemID(ctx, itemID)
	require.NoError(t, err)
	assert.Len(t, bids, 1, "Only successful bid should exist")
	assert.Equal(t, bid1.ID, bids[0].ID)

	// Verify: Item's highest bid is from first bid only
	item, err := svc.ItemRepo.GetItemByID(ctx, itemID)
	require.NoError(t, err)
	assert.Equal(t, int64(100000), item.CurrentHighestBid)

	// Verify: Only one outbox event exists
	tx, err := svc.TxManager.BeginTx(ctx)
	require.NoError(t, err)
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	events, err := svc.OutboxRepo.GetPendingEvents(ctx, tx, 10)
	require.NoError(t, err)
	assert.Len(t, events, 1, "Only one event should exist for the successful bid")
}

// TestAuctionService_PlaceBid_ConcurrentBids_Atomicity tests that concurrent bids
// are handled atomically using SELECT FOR UPDATE locking mechanism
func TestAuctionService_PlaceBid_ConcurrentBids_Atomicity(t *testing.T) {
	// Setup
	testDB := testhelpers.NewTestDatabase(t, "../../../migrations")
	defer testDB.Close()

	pool := testDB.Pool
	svc := setupAuctionService(pool)

	// Seed item
	itemID := uuid.New()
	testItem := &items.Item{
		ID:                itemID,
		Title:             "Test Item",
		StartPrice:        50000,
		CurrentHighestBid: 50000,
		EndAt:             time.Now().Add(24 * time.Hour),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	seedTestItem(t, pool, testItem)

	ctx := context.Background()

	// Launch 10 concurrent bids
	numBids := 10
	var wg sync.WaitGroup
	results := make(chan error, numBids)

	// All bids are for incrementally higher amounts
	for i := 0; i < numBids; i++ {
		wg.Add(1)
		go func(bidAmount int64) {
			defer wg.Done()
			cmd := bids.PlaceBidCommand{
				ItemID: itemID,
				UserID: uuid.New(),
				Amount: bidAmount,
			}
			_, err := svc.Service.PlaceBid(ctx, cmd)
			results <- err
		}(int64(60000 + i*10000)) // $600, $700, $800, etc.
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(results)

	// Collect results
	var successCount, failCount int
	for err := range results {
		if err == nil {
			successCount++
		} else {
			failCount++
		}
	}

	// Verify: All bids should have been processed
	t.Logf("Successful bids: %d, Failed bids: %d", successCount, failCount)
	assert.Equal(t, numBids, successCount+failCount, "All bids should be processed")

	// Verify: Check final state consistency
	item, err := svc.ItemRepo.GetItemByID(ctx, itemID)
	require.NoError(t, err)

	// The highest bid should be the maximum amount
	expectedHighest := int64(60000 + (numBids-1)*10000)
	assert.Equal(t, expectedHighest, item.CurrentHighestBid,
		"Item should have the highest bid amount")

	// Verify: All successful bids are in the database
	bids, err := svc.BidRepo.GetBidsByItemID(ctx, itemID)
	require.NoError(t, err)
	assert.Equal(t, successCount, len(bids),
		"Database should contain all successful bids")

	// Verify: Check for duplicate bids or inconsistencies
	bidAmounts := make(map[int64]int)
	for _, bid := range bids {
		bidAmounts[bid.Amount]++
	}
	for amount, count := range bidAmounts {
		assert.Equal(t, 1, count,
			"Each bid amount should appear exactly once, but %d appears %d times",
			amount, count)
	}

	// Verify: Number of outbox events matches successful bids
	tx, err := svc.TxManager.BeginTx(ctx)
	require.NoError(t, err)
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	events, err := svc.OutboxRepo.GetPendingEvents(ctx, tx, 100)
	require.NoError(t, err)
	assert.Equal(t, successCount, len(events),
		"Number of outbox events should match successful bids")
}

// TestAuctionService_PlaceBid_RaceCondition_SameAmount tests that when two identical
// bids try to update the item simultaneously, the system maintains consistency
func TestAuctionService_PlaceBid_RaceCondition_SameAmount(t *testing.T) {
	// Setup
	testDB := testhelpers.NewTestDatabase(t, "../../../migrations")
	defer testDB.Close()

	pool := testDB.Pool
	svc := setupAuctionService(pool)

	itemID := uuid.New()
	testItem := &items.Item{
		ID:                itemID,
		Title:             "Test Item",
		StartPrice:        50000,
		CurrentHighestBid: 50000,
		EndAt:             time.Now().Add(24 * time.Hour),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	seedTestItem(t, pool, testItem)

	ctx := context.Background()

	// Launch 2 identical bids simultaneously
	var wg sync.WaitGroup
	results := make(chan error, 2)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := bids.PlaceBidCommand{
				ItemID: itemID,
				UserID: uuid.New(),
				Amount: 100000, // SAME amount
			}
			_, err := svc.Service.PlaceBid(ctx, cmd)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	// At least one should succeed
	var successCount int
	for err := range results {
		if err == nil {
			successCount++
		}
	}

	assert.GreaterOrEqual(t, successCount, 1,
		"At least one bid should succeed")

	// Verify database consistency
	bids, err := svc.BidRepo.GetBidsByItemID(ctx, itemID)
	require.NoError(t, err)

	// Due to SELECT FOR UPDATE, exactly one or both should succeed
	// depending on timing, but state should be consistent
	item, err := svc.ItemRepo.GetItemByID(ctx, itemID)
	require.NoError(t, err)
	assert.Equal(t, int64(100000), item.CurrentHighestBid)
	assert.Equal(t, successCount, len(bids),
		"Number of saved bids should match successful operations")

	// Verify: Number of outbox events matches successful bids
	tx, err := svc.TxManager.BeginTx(ctx)
	require.NoError(t, err)
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	events, err := svc.OutboxRepo.GetPendingEvents(ctx, tx, 10)
	require.NoError(t, err)
	assert.Equal(t, successCount, len(events),
		"Number of outbox events should match successful bids")
}
