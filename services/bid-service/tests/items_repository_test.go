package tests

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/floroz/gavel/services/bid-service/internal/adapters/database"
	"github.com/floroz/gavel/services/bid-service/internal/domain/items"
)

func setupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := os.Getenv("BID_DB_URL")
	if dbURL == "" {
		t.Skip("BID_DB_URL not set, skipping integration tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "Failed to connect to database")

	// Clean up tables before each test
	_, err = pool.Exec(ctx, "TRUNCATE items, bids, outbox_events CASCADE")
	require.NoError(t, err, "Failed to truncate tables")

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

func TestItemRepository_CreateItem(t *testing.T) {
	pool := setupTestDB(t)
	repo := database.NewPostgresItemRepository(pool)
	ctx := context.Background()

	item := &items.Item{
		ID:                uuid.New(),
		Title:             "Test Item",
		Description:       "Test Description",
		StartPrice:        1000,
		CurrentHighestBid: 0,
		EndAt:             time.Now().Add(24 * time.Hour),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		Images:            []string{"image1.jpg", "image2.jpg"},
		Category:          "electronics",
		SellerID:          uuid.New(),
		Status:            items.ItemStatusActive,
	}

	err := repo.CreateItem(ctx, item)
	require.NoError(t, err, "Failed to create item")

	// Verify item was created
	retrieved, err := repo.GetItemByID(ctx, item.ID)
	require.NoError(t, err, "Failed to retrieve item")
	assert.Equal(t, item.ID, retrieved.ID)
	assert.Equal(t, item.Title, retrieved.Title)
	assert.Equal(t, item.Description, retrieved.Description)
	assert.Equal(t, item.StartPrice, retrieved.StartPrice)
	assert.Equal(t, item.Images, retrieved.Images)
	assert.Equal(t, item.Category, retrieved.Category)
	assert.Equal(t, item.SellerID, retrieved.SellerID)
	assert.Equal(t, item.Status, retrieved.Status)
}

func TestItemRepository_GetItemByID(t *testing.T) {
	pool := setupTestDB(t)
	repo := database.NewPostgresItemRepository(pool)
	ctx := context.Background()

	t.Run("get existing item", func(t *testing.T) {
		item := &items.Item{
			ID:          uuid.New(),
			Title:       "Test Item",
			Description: "Test Description",
			StartPrice:  1000,
			EndAt:       time.Now().Add(24 * time.Hour),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Images:      []string{},
			SellerID:    uuid.New(),
			Status:      items.ItemStatusActive,
		}

		err := repo.CreateItem(ctx, item)
		require.NoError(t, err)

		retrieved, err := repo.GetItemByID(ctx, item.ID)
		require.NoError(t, err)
		assert.Equal(t, item.ID, retrieved.ID)
		assert.Equal(t, item.Title, retrieved.Title)
	})

	t.Run("get non-existent item", func(t *testing.T) {
		nonExistentID := uuid.New()
		_, err := repo.GetItemByID(ctx, nonExistentID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "item not found")
	})
}

func TestItemRepository_UpdateItem(t *testing.T) {
	pool := setupTestDB(t)
	repo := database.NewPostgresItemRepository(pool)
	ctx := context.Background()

	// Create initial item
	item := &items.Item{
		ID:          uuid.New(),
		Title:       "Original Title",
		Description: "Original Description",
		StartPrice:  1000,
		EndAt:       time.Now().Add(24 * time.Hour),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Images:      []string{"old_image.jpg"},
		Category:    "old_category",
		SellerID:    uuid.New(),
		Status:      items.ItemStatusActive,
	}

	err := repo.CreateItem(ctx, item)
	require.NoError(t, err)

	// Update item
	item.Title = "Updated Title"
	item.Description = "Updated Description"
	item.Images = []string{"new_image1.jpg", "new_image2.jpg"}
	item.Category = "new_category"
	item.UpdatedAt = time.Now()

	err = repo.UpdateItem(ctx, item)
	require.NoError(t, err)

	// Verify updates
	retrieved, err := repo.GetItemByID(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", retrieved.Title)
	assert.Equal(t, "Updated Description", retrieved.Description)
	assert.Equal(t, []string{"new_image1.jpg", "new_image2.jpg"}, retrieved.Images)
	assert.Equal(t, "new_category", retrieved.Category)
}

func TestItemRepository_UpdateStatus(t *testing.T) {
	pool := setupTestDB(t)
	repo := database.NewPostgresItemRepository(pool)
	ctx := context.Background()

	item := &items.Item{
		ID:        uuid.New(),
		Title:     "Test Item",
		StartPrice: 1000,
		EndAt:     time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Images:    []string{},
		SellerID:  uuid.New(),
		Status:    items.ItemStatusActive,
	}

	err := repo.CreateItem(ctx, item)
	require.NoError(t, err)

	// Update status to cancelled
	err = repo.UpdateStatus(ctx, item.ID, items.ItemStatusCancelled)
	require.NoError(t, err)

	// Verify status was updated
	retrieved, err := repo.GetItemByID(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, items.ItemStatusCancelled, retrieved.Status)
}

func TestItemRepository_ListActiveItems(t *testing.T) {
	pool := setupTestDB(t)
	repo := database.NewPostgresItemRepository(pool)
	ctx := context.Background()

	sellerID := uuid.New()

	// Create active items
	for i := 0; i < 3; i++ {
		item := &items.Item{
			ID:         uuid.New(),
			Title:      "Active Item",
			StartPrice: 1000,
			EndAt:      time.Now().Add(24 * time.Hour),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Images:     []string{},
			SellerID:   sellerID,
			Status:     items.ItemStatusActive,
		}
		err := repo.CreateItem(ctx, item)
		require.NoError(t, err)
	}

	// Create ended item
	endedItem := &items.Item{
		ID:         uuid.New(),
		Title:      "Ended Item",
		StartPrice: 1000,
		EndAt:      time.Now().Add(-1 * time.Hour), // Past end time
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Images:     []string{},
		SellerID:   sellerID,
		Status:     items.ItemStatusActive,
	}
	err := repo.CreateItem(ctx, endedItem)
	require.NoError(t, err)

	// Create cancelled item
	cancelledItem := &items.Item{
		ID:         uuid.New(),
		Title:      "Cancelled Item",
		StartPrice: 1000,
		EndAt:      time.Now().Add(24 * time.Hour),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Images:     []string{},
		SellerID:   sellerID,
		Status:     items.ItemStatusCancelled,
	}
	err = repo.CreateItem(ctx, cancelledItem)
	require.NoError(t, err)

	// List active items - should only return 3 active items with future end times
	activeItems, err := repo.ListActiveItems(ctx, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, len(activeItems))
	for _, item := range activeItems {
		assert.Equal(t, items.ItemStatusActive, item.Status)
		assert.True(t, item.EndAt.After(time.Now()))
	}
}

func TestItemRepository_ListItemsBySellerID(t *testing.T) {
	pool := setupTestDB(t)
	repo := database.NewPostgresItemRepository(pool)
	ctx := context.Background()

	seller1ID := uuid.New()
	seller2ID := uuid.New()

	// Create items for seller1
	for i := 0; i < 3; i++ {
		item := &items.Item{
			ID:         uuid.New(),
			Title:      "Seller 1 Item",
			StartPrice: 1000,
			EndAt:      time.Now().Add(24 * time.Hour),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Images:     []string{},
			SellerID:   seller1ID,
			Status:     items.ItemStatusActive,
		}
		err := repo.CreateItem(ctx, item)
		require.NoError(t, err)
	}

	// Create items for seller2
	for i := 0; i < 2; i++ {
		item := &items.Item{
			ID:         uuid.New(),
			Title:      "Seller 2 Item",
			StartPrice: 1000,
			EndAt:      time.Now().Add(24 * time.Hour),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Images:     []string{},
			SellerID:   seller2ID,
			Status:     items.ItemStatusActive,
		}
		err := repo.CreateItem(ctx, item)
		require.NoError(t, err)
	}

	// List seller1's items
	seller1Items, err := repo.ListItemsBySellerID(ctx, seller1ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, len(seller1Items))
	for _, item := range seller1Items {
		assert.Equal(t, seller1ID, item.SellerID)
	}

	// List seller2's items
	seller2Items, err := repo.ListItemsBySellerID(ctx, seller2ID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, len(seller2Items))
	for _, item := range seller2Items {
		assert.Equal(t, seller2ID, item.SellerID)
	}
}

func TestItemRepository_CountBidsByItemID(t *testing.T) {
	pool := setupTestDB(t)
	repo := database.NewPostgresItemRepository(pool)
	ctx := context.Background()

	// Create an item
	item := &items.Item{
		ID:         uuid.New(),
		Title:      "Test Item",
		StartPrice: 1000,
		EndAt:      time.Now().Add(24 * time.Hour),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Images:     []string{},
		SellerID:   uuid.New(),
		Status:     items.ItemStatusActive,
	}
	err := repo.CreateItem(ctx, item)
	require.NoError(t, err)

	// Count bids (should be 0 initially)
	count, err := repo.CountBidsByItemID(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Create some bids directly in DB
	for i := 0; i < 3; i++ {
		_, err = pool.Exec(ctx, `
			INSERT INTO bids (id, item_id, user_id, amount, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`, uuid.New(), item.ID, uuid.New(), int64(1000+i*100), time.Now())
		require.NoError(t, err)
	}

	// Count bids again
	count, err = repo.CountBidsByItemID(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}
