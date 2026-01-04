package tests

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/floroz/gavel/pkg/auth"
	"github.com/floroz/gavel/pkg/database"
	"github.com/floroz/gavel/pkg/proto/bids/v1/bidsv1connect"
	"github.com/floroz/gavel/services/bid-service/internal/adapters/api"
	infradb "github.com/floroz/gavel/services/bid-service/internal/adapters/database"
	"github.com/floroz/gavel/services/bid-service/internal/domain/bids"
	"github.com/floroz/gavel/services/bid-service/internal/domain/items"
)

// testAuthConfig holds the auth configuration for tests
type testAuthConfig struct {
	signer *auth.Signer
}

// generateTestKeys creates RSA key pairs for testing
func generateTestKeys(t *testing.T) ([]byte, []byte) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "Failed to generate RSA key")

	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	})

	pubBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err, "Failed to marshal public key")
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	return privPEM, pubPEM
}

// setupBidApp wires up the application for testing using a real database connection.
// It returns a ConnectRPC client, the database pool, and the auth config for generating tokens.
func setupBidApp(t *testing.T, pool *pgxpool.Pool) (bidsv1connect.BidServiceClient, *pgxpool.Pool, *testAuthConfig) {
	// 1. Generate test keys and create signer
	privPEM, pubPEM := generateTestKeys(t)
	signer, err := auth.NewSigner(privPEM, pubPEM, "test-issuer")
	require.NoError(t, err, "Failed to create signer")

	// 2. Initialize Repositories (Infrastructure Layer)
	txManager := database.NewPostgresTransactionManager(pool, 5*time.Second)
	bidRepo := infradb.NewPostgresBidRepository(pool)
	itemRepo := infradb.NewPostgresItemRepository(pool)
	outboxRepo := infradb.NewPostgresOutboxRepository(pool)

	// 3. Initialize Service (Domain Layer)
	auctionService := bids.NewAuctionService(txManager, bidRepo, itemRepo, outboxRepo)

	// 4. Initialize API Handler with auth interceptor (ConnectRPC)
	bidHandler := api.NewBidServiceHandler(auctionService)
	authInterceptor := auth.NewAuthInterceptor(signer)
	path, handler := bidsv1connect.NewBidServiceHandler(
		bidHandler,
		connect.WithInterceptors(authInterceptor),
	)

	// 5. Create a test HTTP server
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	// 6. Create Client
	client := bidsv1connect.NewBidServiceClient(
		server.Client(),
		server.URL,
	)

	return client, pool, &testAuthConfig{signer: signer}
}

// generateTestToken creates a valid JWT token for the given userID
func (c *testAuthConfig) generateTestToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	pair, err := c.signer.GenerateTokens(userID, "test@example.com", "Test User", nil)
	require.NoError(t, err, "Failed to generate test token")
	return pair.AccessToken
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
