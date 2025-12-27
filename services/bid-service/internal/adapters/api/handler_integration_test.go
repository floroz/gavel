//go:build integration

package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/floroz/gavel/pkg/database"
	bidsv1 "github.com/floroz/gavel/pkg/proto/bids/v1"
	"github.com/floroz/gavel/pkg/proto/bids/v1/bidsv1connect"
	"github.com/floroz/gavel/pkg/testhelpers"
	"github.com/floroz/gavel/services/bid-service/internal/adapters/api"
	infradb "github.com/floroz/gavel/services/bid-service/internal/adapters/database"
	"github.com/floroz/gavel/services/bid-service/internal/domain/bids"
	"github.com/floroz/gavel/services/bid-service/internal/domain/items"
)

// setupBidService creates a handler with all dependencies for testing
func setupBidService(t *testing.T, pool *pgxpool.Pool) (bidsv1connect.BidServiceClient, http.Handler) {
	txManager := database.NewPostgresTransactionManager(pool, 5*time.Second)
	bidRepo := infradb.NewPostgresBidRepository(pool)
	itemRepo := infradb.NewPostgresItemRepository(pool)
	outboxRepo := infradb.NewPostgresOutboxRepository(pool)
	service := bids.NewAuctionService(txManager, bidRepo, itemRepo, outboxRepo)
	handler := api.NewBidServiceHandler(service)

	mux := http.NewServeMux()
	path, h := bidsv1connect.NewBidServiceHandler(handler)
	mux.Handle(path, h)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := bidsv1connect.NewBidServiceClient(
		server.Client(),
		server.URL,
	)

	return client, mux
}

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

func TestBidServiceHandler_PlaceBid_Integration(t *testing.T) {
	// Setup DB
	testDB := testhelpers.NewTestDatabase(t, "../../../migrations")
	defer testDB.Close()

	client, _ := setupBidService(t, testDB.Pool)

	t.Run("Success", func(t *testing.T) {
		// Seed Item
		itemID := uuid.New()
		userID := uuid.New()
		testItem := &items.Item{
			ID:                itemID,
			Title:             "Test Integration Item",
			Description:       "Integration Test",
			StartPrice:        1000,
			CurrentHighestBid: 0,
			EndAt:             time.Now().Add(1 * time.Hour),
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		seedTestItem(t, testDB.Pool, testItem)

		// Make Request
		req := connect.NewRequest(&bidsv1.PlaceBidRequest{
			ItemId: itemID.String(),
			UserId: userID.String(),
			Amount: 1500,
		})

		res, err := client.PlaceBid(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, itemID.String(), res.Msg.Bid.ItemId)
		assert.Equal(t, userID.String(), res.Msg.Bid.UserId)
		assert.Equal(t, int64(1500), res.Msg.Bid.Amount)
	})

	t.Run("ItemNotFound", func(t *testing.T) {
		req := connect.NewRequest(&bidsv1.PlaceBidRequest{
			ItemId: uuid.New().String(),
			UserId: uuid.New().String(),
			Amount: 1500,
		})

		_, err := client.PlaceBid(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeInternal, connect.CodeOf(err)) // Currently mapped to Internal, TODO: Map to NotFound
	})

	t.Run("BidTooLow", func(t *testing.T) {
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
		seedTestItem(t, testDB.Pool, testItem)

		req := connect.NewRequest(&bidsv1.PlaceBidRequest{
			ItemId: itemID.String(),
			UserId: uuid.New().String(),
			Amount: 4000,
		})

		_, err := client.PlaceBid(context.Background(), req)
		require.Error(t, err)
		// Currently mapped to Internal, would be nice to map to FailedPrecondition or similar
		assert.Equal(t, connect.CodeInternal, connect.CodeOf(err))
	})
}

