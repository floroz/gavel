package tests

import (
	"context"
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

func TestAPI_CreateItem(t *testing.T) {
	testDB := testhelpers.NewTestDatabase(t, "../migrations")
	defer testDB.Close()

	client, _, authConfig := setupBidApp(t, testDB.Pool)

	userID := uuid.New()
	token := authConfig.generateTestToken(t, userID)
	ctx := context.Background()

	t.Run("successfully creates item", func(t *testing.T) {
		req := &bidsv1.CreateItemRequest{
			Title:       "Test Auction Item",
			Description: "A great item for testing",
			StartPrice:  1000,
			EndAt:       time.Now().Add(48 * time.Hour).Format(time.RFC3339),
			Images:      []string{"image1.jpg", "image2.jpg"},
			Category:    "electronics",
		}

		r := connect.NewRequest(req)
		r.Header().Set("Authorization", "Bearer "+token)
		resp, err := client.CreateItem(ctx, r)
		require.NoError(t, err)
		require.NotNil(t, resp.Msg.Item)

		item := resp.Msg.Item
		assert.NotEmpty(t, item.Id)
		assert.Equal(t, req.Title, item.Title)
		assert.Equal(t, req.Description, item.Description)
		assert.Equal(t, req.StartPrice, item.StartPrice)
		assert.Equal(t, int64(0), item.CurrentHighestBid)
		assert.Equal(t, req.Images, item.Images)
		assert.Equal(t, req.Category, item.Category)
		assert.Equal(t, userID.String(), item.SellerId)
		assert.Equal(t, bidsv1.ItemStatus_ITEM_STATUS_ACTIVE, item.Status)
	})

	t.Run("fails with invalid start price", func(t *testing.T) {
		req := &bidsv1.CreateItemRequest{
			Title:      "Invalid Item",
			StartPrice: 0,
			EndAt:      time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		}

		r := connect.NewRequest(req)
		r.Header().Set("Authorization", "Bearer "+token)
		_, err := client.CreateItem(ctx, r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "start price")
	})

	t.Run("fails with past end time", func(t *testing.T) {
		req := &bidsv1.CreateItemRequest{
			Title:      "Invalid Item",
			StartPrice: 1000,
			EndAt:      time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		}

		r := connect.NewRequest(req)
		r.Header().Set("Authorization", "Bearer "+token)
		_, err := client.CreateItem(ctx, r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "end time")
	})

	t.Run("fails without authentication", func(t *testing.T) {
		req := &bidsv1.CreateItemRequest{
			Title:      "Test Item",
			StartPrice: 1000,
			EndAt:      time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		}

		_, err := client.CreateItem(ctx, connect.NewRequest(req))
		require.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})
}

func TestAPI_GetItem(t *testing.T) {
	testDB := testhelpers.NewTestDatabase(t, "../migrations")
	defer testDB.Close()

	client, pool, _ := setupBidApp(t, testDB.Pool)
	ctx := context.Background()

	// Seed an item
	item := &items.Item{
		ID:                uuid.New(),
		Title:             "Test Item",
		Description:       "Test Description",
		StartPrice:        1000,
		CurrentHighestBid: 0,
		EndAt:             time.Now().Add(24 * time.Hour),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		Images:            []string{"image1.jpg"},
		Category:          "electronics",
		SellerID:          uuid.New(),
		Status:            items.ItemStatusActive,
	}
	seedTestItem(t, pool, item)

	t.Run("successfully gets item", func(t *testing.T) {
		req := &bidsv1.GetItemRequest{
			Id: item.ID.String(),
		}

		resp, err := client.GetItem(ctx, connect.NewRequest(req))
		require.NoError(t, err)
		require.NotNil(t, resp.Msg.Item)

		retrieved := resp.Msg.Item
		assert.Equal(t, item.ID.String(), retrieved.Id)
		assert.Equal(t, item.Title, retrieved.Title)
		assert.Equal(t, item.Description, retrieved.Description)
		assert.Equal(t, item.StartPrice, retrieved.StartPrice)
	})

	t.Run("fails with invalid ID", func(t *testing.T) {
		req := &bidsv1.GetItemRequest{
			Id: "invalid-id",
		}

		_, err := client.GetItem(ctx, connect.NewRequest(req))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid id")
	})

	t.Run("fails with non-existent ID", func(t *testing.T) {
		req := &bidsv1.GetItemRequest{
			Id: uuid.New().String(),
		}

		_, err := client.GetItem(ctx, connect.NewRequest(req))
		require.Error(t, err)
		assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	})
}

func TestAPI_ListItems(t *testing.T) {
	testDB := testhelpers.NewTestDatabase(t, "../migrations")
	defer testDB.Close()

	client, pool, _ := setupBidApp(t, testDB.Pool)
	ctx := context.Background()

	sellerID := uuid.New()

	// Seed 3 active items
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
		seedTestItem(t, pool, item)
	}

	// Seed 1 cancelled item (should not appear)
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
	seedTestItem(t, pool, cancelledItem)

	t.Run("successfully lists active items", func(t *testing.T) {
		req := &bidsv1.ListItemsRequest{
			PageSize: 10,
		}

		resp, err := client.ListItems(ctx, connect.NewRequest(req))
		require.NoError(t, err)
		require.NotNil(t, resp.Msg)

		assert.Equal(t, 3, len(resp.Msg.Items))
		for _, item := range resp.Msg.Items {
			assert.Equal(t, bidsv1.ItemStatus_ITEM_STATUS_ACTIVE, item.Status)
		}
	})
}

func TestAPI_ListSellerItems(t *testing.T) {
	testDB := testhelpers.NewTestDatabase(t, "../migrations")
	defer testDB.Close()

	client, pool, authConfig := setupBidApp(t, testDB.Pool)
	ctx := context.Background()

	seller1ID := uuid.New()
	seller2ID := uuid.New()

	// Seed items for seller1
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
		seedTestItem(t, pool, item)
	}

	// Seed items for seller2
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
	seedTestItem(t, pool, item)

	t.Run("successfully lists seller's items", func(t *testing.T) {
		token := authConfig.generateTestToken(t, seller1ID)
		req := &bidsv1.ListSellerItemsRequest{
			PageSize: 10,
		}

		r := connect.NewRequest(req)
		r.Header().Set("Authorization", "Bearer "+token)
		resp, err := client.ListSellerItems(ctx, r)
		require.NoError(t, err)
		require.NotNil(t, resp.Msg)

		assert.Equal(t, 3, len(resp.Msg.Items))
		for _, item := range resp.Msg.Items {
			assert.Equal(t, seller1ID.String(), item.SellerId)
		}
	})

	t.Run("fails without authentication", func(t *testing.T) {
		req := &bidsv1.ListSellerItemsRequest{
			PageSize: 10,
		}

		_, err := client.ListSellerItems(ctx, connect.NewRequest(req))
		require.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})
}

func TestAPI_UpdateItem(t *testing.T) {
	testDB := testhelpers.NewTestDatabase(t, "../migrations")
	defer testDB.Close()

	client, pool, authConfig := setupBidApp(t, testDB.Pool)
	ctx := context.Background()

	ownerID := uuid.New()
	otherUserID := uuid.New()

	// Seed an item
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
		SellerID:    ownerID,
		Status:      items.ItemStatusActive,
	}
	seedTestItem(t, pool, item)

	t.Run("successfully updates item", func(t *testing.T) {
		token := authConfig.generateTestToken(t, ownerID)
		newTitle := "Updated Title"
		newDescription := "Updated Description"
		newCategory := "new_category"

		req := &bidsv1.UpdateItemRequest{
			Id:          item.ID.String(),
			Title:       &newTitle,
			Description: &newDescription,
			Images:      []string{"new_image1.jpg", "new_image2.jpg"},
			Category:    &newCategory,
		}

		r := connect.NewRequest(req)
		r.Header().Set("Authorization", "Bearer "+token)
		resp, err := client.UpdateItem(ctx, r)
		require.NoError(t, err)
		require.NotNil(t, resp.Msg.Item)

		updated := resp.Msg.Item
		assert.Equal(t, newTitle, updated.Title)
		assert.Equal(t, newDescription, updated.Description)
		assert.Equal(t, []string{"new_image1.jpg", "new_image2.jpg"}, updated.Images)
		assert.Equal(t, newCategory, updated.Category)
	})

	t.Run("fails when user is not owner", func(t *testing.T) {
		token := authConfig.generateTestToken(t, otherUserID)
		newTitle := "Unauthorized Update"

		req := &bidsv1.UpdateItemRequest{
			Id:    item.ID.String(),
			Title: &newTitle,
		}

		r := connect.NewRequest(req)
		r.Header().Set("Authorization", "Bearer "+token)
		_, err := client.UpdateItem(ctx, r)
		require.Error(t, err)
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	})

	t.Run("fails without authentication", func(t *testing.T) {
		newTitle := "Unauthorized Update"
		req := &bidsv1.UpdateItemRequest{
			Id:    item.ID.String(),
			Title: &newTitle,
		}

		_, err := client.UpdateItem(ctx, connect.NewRequest(req))
		require.Error(t, err)
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	})
}

func TestAPI_CancelItem(t *testing.T) {
	testDB := testhelpers.NewTestDatabase(t, "../migrations")
	defer testDB.Close()

	client, pool, authConfig := setupBidApp(t, testDB.Pool)
	ctx := context.Background()

	t.Run("successfully cancels item with no bids", func(t *testing.T) {
		ownerID := uuid.New()
		token := authConfig.generateTestToken(t, ownerID)

		// Create item
		item := &items.Item{
			ID:         uuid.New(),
			Title:      "Item to Cancel",
			StartPrice: 1000,
			EndAt:      time.Now().Add(24 * time.Hour),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Images:     []string{},
			SellerID:   ownerID,
			Status:     items.ItemStatusActive,
		}
		seedTestItem(t, pool, item)

		req := &bidsv1.CancelItemRequest{
			Id: item.ID.String(),
		}

		r := connect.NewRequest(req)
		r.Header().Set("Authorization", "Bearer "+token)
		resp, err := client.CancelItem(ctx, r)
		require.NoError(t, err)
		require.NotNil(t, resp.Msg.Item)
		assert.Equal(t, bidsv1.ItemStatus_ITEM_STATUS_CANCELLED, resp.Msg.Item.Status)
	})

	t.Run("fails when item has bids", func(t *testing.T) {
		ownerID := uuid.New()
		token := authConfig.generateTestToken(t, ownerID)

		// Create item with bid
		item := &items.Item{
			ID:         uuid.New(),
			Title:      "Item with Bids",
			StartPrice: 1000,
			EndAt:      time.Now().Add(24 * time.Hour),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Images:     []string{},
			SellerID:   ownerID,
			Status:     items.ItemStatusActive,
		}
		seedTestItem(t, pool, item)

		// Add a bid
		_, err := pool.Exec(ctx, `
			INSERT INTO bids (id, item_id, user_id, amount, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`, uuid.New(), item.ID, uuid.New(), int64(1500), time.Now())
		require.NoError(t, err)

		req := &bidsv1.CancelItemRequest{
			Id: item.ID.String(),
		}

		r := connect.NewRequest(req)
		r.Header().Set("Authorization", "Bearer "+token)
		_, err = client.CancelItem(ctx, r)
		require.Error(t, err)
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
		assert.Contains(t, err.Error(), "cannot cancel")
	})

	t.Run("fails when user is not owner", func(t *testing.T) {
		ownerID := uuid.New()
		otherUserID := uuid.New()
		token := authConfig.generateTestToken(t, otherUserID)

		// Create item
		item := &items.Item{
			ID:         uuid.New(),
			Title:      "Someone Else's Item",
			StartPrice: 1000,
			EndAt:      time.Now().Add(24 * time.Hour),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Images:     []string{},
			SellerID:   ownerID,
			Status:     items.ItemStatusActive,
		}
		seedTestItem(t, pool, item)

		req := &bidsv1.CancelItemRequest{
			Id: item.ID.String(),
		}

		r := connect.NewRequest(req)
		r.Header().Set("Authorization", "Bearer "+token)
		_, err := client.CancelItem(ctx, r)
		require.Error(t, err)
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	})
}

func TestAPI_GetItemBids(t *testing.T) {
	testDB := testhelpers.NewTestDatabase(t, "../migrations")
	defer testDB.Close()

	client, pool, _ := setupBidApp(t, testDB.Pool)
	ctx := context.Background()

	// Create item
	item := &items.Item{
		ID:         uuid.New(),
		Title:      "Item with Bids",
		StartPrice: 1000,
		EndAt:      time.Now().Add(24 * time.Hour),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Images:     []string{},
		SellerID:   uuid.New(),
		Status:     items.ItemStatusActive,
	}
	seedTestItem(t, pool, item)

	// Add some bids
	for i := 0; i < 3; i++ {
		_, err := pool.Exec(ctx, `
			INSERT INTO bids (id, item_id, user_id, amount, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`, uuid.New(), item.ID, uuid.New(), int64(1000+i*100), time.Now())
		require.NoError(t, err)
	}

	t.Run("successfully gets item bids", func(t *testing.T) {
		req := &bidsv1.GetItemBidsRequest{
			ItemId: item.ID.String(),
		}

		resp, err := client.GetItemBids(ctx, connect.NewRequest(req))
		require.NoError(t, err)
		require.NotNil(t, resp.Msg)

		assert.Equal(t, 3, len(resp.Msg.Bids))
	})

	t.Run("returns empty list for item with no bids", func(t *testing.T) {
		itemWithNoBids := &items.Item{
			ID:         uuid.New(),
			Title:      "Item without Bids",
			StartPrice: 1000,
			EndAt:      time.Now().Add(24 * time.Hour),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
			Images:     []string{},
			SellerID:   uuid.New(),
			Status:     items.ItemStatusActive,
		}
		seedTestItem(t, pool, itemWithNoBids)

		req := &bidsv1.GetItemBidsRequest{
			ItemId: itemWithNoBids.ID.String(),
		}

		resp, err := client.GetItemBids(ctx, connect.NewRequest(req))
		require.NoError(t, err)
		require.NotNil(t, resp.Msg)

		assert.Equal(t, 0, len(resp.Msg.Bids))
	})
}

func TestAPI_SellerCannotBidOnOwnItem(t *testing.T) {
	testDB := testhelpers.NewTestDatabase(t, "../migrations")
	defer testDB.Close()

	client, pool, authConfig := setupBidApp(t, testDB.Pool)
	ctx := context.Background()

	sellerID := uuid.New()
	token := authConfig.generateTestToken(t, sellerID)

	// Create item as seller
	item := &items.Item{
		ID:                uuid.New(),
		Title:             "Seller's Item",
		StartPrice:        1000,
		CurrentHighestBid: 0,
		EndAt:             time.Now().Add(24 * time.Hour),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		Images:            []string{},
		SellerID:          sellerID,
		Status:            items.ItemStatusActive,
	}
	seedTestItem(t, pool, item)

	t.Run("seller cannot bid on own item", func(t *testing.T) {
		req := &bidsv1.PlaceBidRequest{
			ItemId: item.ID.String(),
			Amount: 1500,
		}

		r := connect.NewRequest(req)
		r.Header().Set("Authorization", "Bearer "+token)
		_, err := client.PlaceBid(ctx, r)
		require.Error(t, err)
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
		assert.Contains(t, err.Error(), "seller cannot bid")
	})
}
