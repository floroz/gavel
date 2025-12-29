package tests

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/floroz/gavel/pkg/database"
	"github.com/floroz/gavel/pkg/proto/bids/v1/bidsv1connect"
	"github.com/floroz/gavel/services/bid-service/internal/adapters/api"
	infradb "github.com/floroz/gavel/services/bid-service/internal/adapters/database"
	"github.com/floroz/gavel/services/bid-service/internal/domain/bids"
	"github.com/floroz/gavel/services/bid-service/internal/domain/items"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// setupBidApp wires up the application for testing using a real database connection.
// It returns a ConnectRPC client and the database pool for verification.
func setupBidApp(t *testing.T, pool *pgxpool.Pool) (bidsv1connect.BidServiceClient, *pgxpool.Pool) {
	// 1. Initialize Repositories (Infrastructure Layer)
	// We use the same pool passed from the test helper (Testcontainers)
	txManager := database.NewPostgresTransactionManager(pool, 5*time.Second)
	bidRepo := infradb.NewPostgresBidRepository(pool)
	itemRepo := infradb.NewPostgresItemRepository(pool)
	outboxRepo := infradb.NewPostgresOutboxRepository(pool)

	// 2. Initialize Service (Domain Layer)
	auctionService := bids.NewAuctionService(txManager, bidRepo, itemRepo, outboxRepo)

	// 3. Initialize API Handler (ConnectRPC)
	bidHandler := api.NewBidServiceHandler(auctionService)
	path, handler := bidsv1connect.NewBidServiceHandler(bidHandler)

	// 4. Create a test HTTP server
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	// 5. Create Client
	client := bidsv1connect.NewBidServiceClient(
		server.Client(),
		server.URL,
	)

	return client, pool
}

// seedTestItem inserts a test item into the database directly.
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

// getTestItem retrieves an item from the database for verification.
func getTestItem(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) *items.Item {
	t.Helper()
	var item items.Item
	row := pool.QueryRow(context.Background(), "SELECT current_highest_bid FROM items WHERE id = $1", id)
	err := row.Scan(&item.CurrentHighestBid)
	require.NoError(t, err)
	return &item
}

// countOutboxEvents counts the number of events in the outbox table.
func countOutboxEvents(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var count int
	row := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM outbox_events")
	_ = row.Scan(&count)
	return count
}
