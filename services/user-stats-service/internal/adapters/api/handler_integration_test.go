package api_test

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/floroz/gavel/pkg/auth"
	"github.com/floroz/gavel/pkg/database"
	userstatsv1 "github.com/floroz/gavel/pkg/proto/userstats/v1"
	"github.com/floroz/gavel/pkg/proto/userstats/v1/userstatsv1connect"
	"github.com/floroz/gavel/pkg/testhelpers"
	"github.com/floroz/gavel/services/user-stats-service/internal/adapters/api"
	infradb "github.com/floroz/gavel/services/user-stats-service/internal/adapters/database"
	"github.com/floroz/gavel/services/user-stats-service/internal/domain/userstats"
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

// generateTestToken creates a valid JWT token for the given userID
func (c *testAuthConfig) generateTestToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()
	pair, err := c.signer.GenerateTokens(userID, "test@example.com", "Test User", nil)
	require.NoError(t, err, "Failed to generate test token")
	return pair.AccessToken
}

// setupUserStatsService creates a handler with all dependencies for testing
func setupUserStatsService(t *testing.T, pool *pgxpool.Pool) (userstatsv1connect.UserStatsServiceClient, http.Handler, *testAuthConfig) {
	// Generate test keys and create signer
	privPEM, pubPEM := generateTestKeys(t)
	signer, err := auth.NewSigner(privPEM, pubPEM, "test-issuer")
	require.NoError(t, err, "Failed to create signer")

	txManager := database.NewPostgresTransactionManager(pool, 5*time.Second)
	repo := infradb.NewUserStatsRepository(pool)
	service := userstats.NewService(repo, txManager)
	handler := api.NewUserStatsServiceHandler(service)

	mux := http.NewServeMux()
	authInterceptor := auth.NewAuthInterceptor(signer)
	path, h := userstatsv1connect.NewUserStatsServiceHandler(
		handler,
		connect.WithInterceptors(authInterceptor),
	)
	mux.Handle(path, h)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := userstatsv1connect.NewUserStatsServiceClient(
		server.Client(),
		server.URL,
	)

	return client, mux, &testAuthConfig{signer: signer}
}

// seedUserStats inserts test stats into the database
func seedUserStats(t *testing.T, pool *pgxpool.Pool, stats *userstats.UserStats) {
	t.Helper()
	ctx := context.Background()
	query := `
		INSERT INTO user_stats (user_id, total_bids_placed, total_amount_bid, last_bid_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := pool.Exec(ctx, query,
		stats.UserID,
		stats.TotalBidsPlaced,
		stats.TotalAmountBid,
		stats.LastBidAt,
		stats.CreatedAt,
		stats.UpdatedAt,
	)
	require.NoError(t, err, "Failed to seed user stats")
}

func TestUserStatsServiceHandler_GetUserStats_Integration(t *testing.T) {
	// Setup DB
	testDB := testhelpers.NewTestDatabase(t, "../../../migrations")
	defer testDB.Close()

	client, _, authConfig := setupUserStatsService(t, testDB.Pool)

	t.Run("Success", func(t *testing.T) {
		// Seed Stats
		userID := uuid.New()
		expectedStats := &userstats.UserStats{
			UserID:          userID,
			TotalBidsPlaced: 5,
			TotalAmountBid:  5000,
			LastBidAt:       time.Now().Truncate(time.Second), // Truncate for DB precision comparison
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
		seedUserStats(t, testDB.Pool, expectedStats)

		// Make Request with auth header
		req := connect.NewRequest(&userstatsv1.GetUserStatsRequest{})
		req.Header().Set("Authorization", "Bearer "+authConfig.generateTestToken(t, userID))

		res, err := client.GetUserStats(context.Background(), req)
		require.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, userID.String(), res.Msg.Stats.UserId)
		assert.Equal(t, int64(5), res.Msg.Stats.TotalBids)
		assert.Equal(t, int64(5000), res.Msg.Stats.TotalAmount)

		// Parse returned time to compare
		returnedTime, err := time.Parse(time.RFC3339, res.Msg.Stats.LastUpdatedAt)
		require.NoError(t, err)
		assert.WithinDuration(t, expectedStats.LastBidAt, returnedTime, time.Second)
	})

	t.Run("NotFound", func(t *testing.T) {
		// Request for a user that has no stats
		userID := uuid.New()
		req := connect.NewRequest(&userstatsv1.GetUserStatsRequest{})
		req.Header().Set("Authorization", "Bearer "+authConfig.generateTestToken(t, userID))

		_, err := client.GetUserStats(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		// Request without auth header
		req := connect.NewRequest(&userstatsv1.GetUserStatsRequest{})

		_, err := client.GetUserStats(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})
}
