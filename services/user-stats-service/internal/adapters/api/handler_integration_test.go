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
	userstatsv1 "github.com/floroz/gavel/pkg/proto/userstats/v1"
	"github.com/floroz/gavel/pkg/proto/userstats/v1/userstatsv1connect"
	"github.com/floroz/gavel/pkg/testhelpers"
	"github.com/floroz/gavel/services/user-stats-service/internal/adapters/api"
	infradb "github.com/floroz/gavel/services/user-stats-service/internal/adapters/database"
	"github.com/floroz/gavel/services/user-stats-service/internal/domain/userstats"
)

// setupUserStatsService creates a handler with all dependencies for testing
func setupUserStatsService(t *testing.T, pool *pgxpool.Pool) (userstatsv1connect.UserStatsServiceClient, http.Handler) {
	txManager := database.NewPostgresTransactionManager(pool, 5*time.Second)
	repo := infradb.NewUserStatsRepository(pool)
	service := userstats.NewService(repo, txManager)
	handler := api.NewUserStatsServiceHandler(service)

	mux := http.NewServeMux()
	path, h := userstatsv1connect.NewUserStatsServiceHandler(handler)
	mux.Handle(path, h)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := userstatsv1connect.NewUserStatsServiceClient(
		server.Client(),
		server.URL,
	)

	return client, mux
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

	client, _ := setupUserStatsService(t, testDB.Pool)

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

		// Make Request
		req := connect.NewRequest(&userstatsv1.GetUserStatsRequest{
			UserId: userID.String(),
		})

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
		req := connect.NewRequest(&userstatsv1.GetUserStatsRequest{
			UserId: uuid.New().String(),
		})

		_, err := client.GetUserStats(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	})

	t.Run("InvalidID", func(t *testing.T) {
		req := connect.NewRequest(&userstatsv1.GetUserStatsRequest{
			UserId: "invalid-uuid",
		})

		_, err := client.GetUserStats(context.Background(), req)
		require.Error(t, err)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	})
}
