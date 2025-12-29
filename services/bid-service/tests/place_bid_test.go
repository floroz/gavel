package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bidsv1 "github.com/floroz/gavel/pkg/proto/bids/v1"
	"github.com/floroz/gavel/pkg/testhelpers"
	"github.com/floroz/gavel/services/bid-service/internal/domain/items"
)

func TestPlaceBid_Scenarios(t *testing.T) {
	// Setup DB Container
	testDB := testhelpers.NewTestDatabase(t, "../migrations")
	defer testDB.Close()

	// Setup Application
	client, pool := setupBidApp(t, testDB.Pool)

	t.Run("Success_ValidBid", func(t *testing.T) {
		// Arrange
		itemID := uuid.New()
		userID := uuid.New()
		testItem := &items.Item{
			ID:                itemID,
			Title:             "Integration Item",
			StartPrice:        1000,
			CurrentHighestBid: 0,
			EndAt:             time.Now().Add(1 * time.Hour),
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		seedTestItem(t, pool, testItem)

		// Act
		req := connect.NewRequest(&bidsv1.PlaceBidRequest{
			ItemId: itemID.String(),
			UserId: userID.String(),
			Amount: 1500,
		})
		res, err := client.PlaceBid(context.Background(), req)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, itemID.String(), res.Msg.Bid.ItemId)
		assert.Equal(t, int64(1500), res.Msg.Bid.Amount)

		// Verify DB State
		updatedItem := getTestItem(t, pool, itemID)
		assert.Equal(t, int64(1500), updatedItem.CurrentHighestBid)
	})

	t.Run("Failure_ItemNotFound", func(t *testing.T) {
		req := connect.NewRequest(&bidsv1.PlaceBidRequest{
			ItemId: uuid.New().String(),
			UserId: uuid.New().String(),
			Amount: 1500,
		})

		_, err := client.PlaceBid(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	})

	t.Run("Failure_BidTooLow", func(t *testing.T) {
		itemID := uuid.New()
		testItem := &items.Item{
			ID:                itemID,
			Title:             "Expensive Item",
			StartPrice:        5000,
			CurrentHighestBid: 5000,
			EndAt:             time.Now().Add(1 * time.Hour),
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		seedTestItem(t, pool, testItem)

		req := connect.NewRequest(&bidsv1.PlaceBidRequest{
			ItemId: itemID.String(),
			UserId: uuid.New().String(),
			Amount: 4000,
		})

		_, err := client.PlaceBid(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	})

	t.Run("Failure_AuctionEnded", func(t *testing.T) {
		itemID := uuid.New()
		testItem := &items.Item{
			ID:                itemID,
			Title:             "Ended Auction",
			StartPrice:        1000,
			CurrentHighestBid: 2000,
			EndAt:             time.Now().Add(-1 * time.Hour),
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		seedTestItem(t, pool, testItem)

		req := connect.NewRequest(&bidsv1.PlaceBidRequest{
			ItemId: itemID.String(),
			UserId: uuid.New().String(),
			Amount: 3000,
		})

		_, err := client.PlaceBid(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	})

	t.Run("Failure_NegativeAmount", func(t *testing.T) {
		itemID := uuid.New()
		testItem := &items.Item{
			ID:                itemID,
			Title:             "Item for Negative Bid",
			StartPrice:        1000,
			CurrentHighestBid: 1000,
			EndAt:             time.Now().Add(1 * time.Hour),
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		seedTestItem(t, pool, testItem)

		req := connect.NewRequest(&bidsv1.PlaceBidRequest{
			ItemId: itemID.String(),
			UserId: uuid.New().String(),
			Amount: -100,
		})
		_, err := client.PlaceBid(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("Failure_ZeroAmount", func(t *testing.T) {
		itemID := uuid.New()
		testItem := &items.Item{
			ID:                itemID,
			Title:             "Item for Zero Bid",
			StartPrice:        1000,
			CurrentHighestBid: 1000,
			EndAt:             time.Now().Add(1 * time.Hour),
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		seedTestItem(t, pool, testItem)

		req := connect.NewRequest(&bidsv1.PlaceBidRequest{
			ItemId: itemID.String(),
			UserId: uuid.New().String(),
			Amount: 0,
		})
		_, err := client.PlaceBid(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})

	t.Run("Concurrency_Atomicity", func(t *testing.T) {
		// Simulating multiple users bidding on the same item rapidly
		itemID := uuid.New()
		testItem := &items.Item{
			ID:                itemID,
			Title:             "Concurrent Item",
			StartPrice:        50000,
			CurrentHighestBid: 50000,
			EndAt:             time.Now().Add(24 * time.Hour),
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		seedTestItem(t, pool, testItem)

		numBids := 10
		var wg sync.WaitGroup
		results := make(chan error, numBids)

		// Launch 10 bids in parallel, each increasing by 1000
		for i := 0; i < numBids; i++ {
			wg.Add(1)
			go func(amount int64) {
				defer wg.Done()
				req := connect.NewRequest(&bidsv1.PlaceBidRequest{
					ItemId: itemID.String(),
					UserId: uuid.New().String(),
					Amount: amount,
				})
				_, err := client.PlaceBid(context.Background(), req)
				results <- err
			}(int64(60000 + i*1000))
		}

		wg.Wait()
		close(results)

		// Verify final state
		updatedItem := getTestItem(t, pool, itemID)
		expectedMax := int64(60000 + (numBids-1)*1000)
		assert.Equal(t, expectedMax, updatedItem.CurrentHighestBid)
	})

	t.Run("Concurrency_RaceCondition_SameAmount", func(t *testing.T) {
		// Two users bid the SAME amount at the SAME time.
		// Only one should succeed.
		itemID := uuid.New()
		testItem := &items.Item{
			ID:                itemID,
			Title:             "Race Item",
			StartPrice:        50000,
			CurrentHighestBid: 50000,
			EndAt:             time.Now().Add(24 * time.Hour),
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		seedTestItem(t, pool, testItem)

		var wg sync.WaitGroup
		results := make(chan error, 2)

		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				req := connect.NewRequest(&bidsv1.PlaceBidRequest{
					ItemId: itemID.String(),
					UserId: uuid.New().String(),
					Amount: 60000,
				})
				_, err := client.PlaceBid(context.Background(), req)
				results <- err
			}()
		}

		wg.Wait()
		close(results)

		var successCount int
		for err := range results {
			if err == nil {
				successCount++
			}
		}

		assert.Equal(t, 1, successCount, "Only one bid should succeed for the same amount")
	})
}
