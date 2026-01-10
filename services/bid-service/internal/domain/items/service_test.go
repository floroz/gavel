package items

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRepository is a mock implementation of Repository for testing
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateItem(ctx context.Context, item *Item) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockRepository) GetItemByID(ctx context.Context, itemID uuid.UUID) (*Item, error) {
	args := m.Called(ctx, itemID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Item), args.Error(1)
}

func (m *MockRepository) GetItemByIDForUpdate(ctx context.Context, tx pgx.Tx, itemID uuid.UUID) (*Item, error) {
	args := m.Called(ctx, tx, itemID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Item), args.Error(1)
}

func (m *MockRepository) UpdateItem(ctx context.Context, item *Item) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockRepository) UpdateStatus(ctx context.Context, itemID uuid.UUID, status ItemStatus) error {
	args := m.Called(ctx, itemID, status)
	return args.Error(0)
}

func (m *MockRepository) UpdateHighestBid(ctx context.Context, tx pgx.Tx, itemID uuid.UUID, amount int64) error {
	args := m.Called(ctx, tx, itemID, amount)
	return args.Error(0)
}

func (m *MockRepository) ListActiveItems(ctx context.Context, limit, offset int) ([]*Item, error) {
	args := m.Called(ctx, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Item), args.Error(1)
}

func (m *MockRepository) ListItemsBySellerID(ctx context.Context, sellerID uuid.UUID, limit, offset int) ([]*Item, error) {
	args := m.Called(ctx, sellerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Item), args.Error(1)
}

func (m *MockRepository) CountBidsByItemID(ctx context.Context, itemID uuid.UUID) (int64, error) {
	args := m.Called(ctx, itemID)
	return args.Get(0).(int64), args.Error(1)
}

func TestService_CreateItem(t *testing.T) {
	tests := []struct {
		name        string
		cmd         CreateItemCommand
		setupMock   func(*MockRepository)
		wantErr     error
		checkResult func(*testing.T, *Item)
	}{
		{
			name: "successfully creates item",
			cmd: CreateItemCommand{
				Title:       "Test Item",
				Description: "Test Description",
				StartPrice:  1000,
				EndAt:       time.Now().Add(24 * time.Hour),
				Images:      []string{"image1.jpg"},
				Category:    "electronics",
				SellerID:    uuid.New(),
			},
			setupMock: func(repo *MockRepository) {
				repo.On("CreateItem", mock.Anything, mock.AnythingOfType("*items.Item")).Return(nil)
			},
			wantErr: nil,
			checkResult: func(t *testing.T, item *Item) {
				assert.NotEqual(t, uuid.Nil, item.ID)
				assert.Equal(t, "Test Item", item.Title)
				assert.Equal(t, int64(1000), item.StartPrice)
				assert.Equal(t, ItemStatusActive, item.Status)
				assert.Equal(t, int64(0), item.CurrentHighestBid)
			},
		},
		{
			name: "fails with invalid start price (zero)",
			cmd: CreateItemCommand{
				Title:      "Test Item",
				StartPrice: 0,
				EndAt:      time.Now().Add(24 * time.Hour),
				SellerID:   uuid.New(),
			},
			setupMock: func(repo *MockRepository) {
				// No repo calls expected
			},
			wantErr: ErrInvalidStartPrice,
		},
		{
			name: "fails with invalid start price (negative)",
			cmd: CreateItemCommand{
				Title:      "Test Item",
				StartPrice: -100,
				EndAt:      time.Now().Add(24 * time.Hour),
				SellerID:   uuid.New(),
			},
			setupMock: func(repo *MockRepository) {
				// No repo calls expected
			},
			wantErr: ErrInvalidStartPrice,
		},
		{
			name: "fails with end time in past",
			cmd: CreateItemCommand{
				Title:      "Test Item",
				StartPrice: 1000,
				EndAt:      time.Now().Add(-1 * time.Hour),
				SellerID:   uuid.New(),
			},
			setupMock: func(repo *MockRepository) {
				// No repo calls expected
			},
			wantErr: ErrInvalidEndTime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockRepository)
			tt.setupMock(repo)

			service := NewService(repo)
			item, err := service.CreateItem(context.Background(), tt.cmd)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, item)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, item)
				if tt.checkResult != nil {
					tt.checkResult(t, item)
				}
			}

			repo.AssertExpectations(t)
		})
	}
}

func TestService_UpdateItem(t *testing.T) {
	itemID := uuid.New()
	ownerID := uuid.New()
	otherUserID := uuid.New()

	tests := []struct {
		name      string
		cmd       UpdateItemCommand
		setupMock func(*MockRepository)
		wantErr   error
	}{
		{
			name: "successfully updates item",
			cmd: UpdateItemCommand{
				ItemID:      itemID,
				UserID:      ownerID,
				Title:       "Updated Title",
				Description: "Updated Description",
				Images:      []string{"new_image.jpg"},
				Category:    "new_category",
			},
			setupMock: func(repo *MockRepository) {
				repo.On("GetItemByID", mock.Anything, itemID).Return(&Item{
					ID:       itemID,
					SellerID: ownerID,
					Title:    "Old Title",
				}, nil)
				repo.On("UpdateItem", mock.Anything, mock.AnythingOfType("*items.Item")).Return(nil)
			},
			wantErr: nil,
		},
		{
			name: "fails when item not found",
			cmd: UpdateItemCommand{
				ItemID: itemID,
				UserID: ownerID,
			},
			setupMock: func(repo *MockRepository) {
				repo.On("GetItemByID", mock.Anything, itemID).Return(nil, errors.New("not found"))
			},
			wantErr: ErrItemNotFound,
		},
		{
			name: "fails when user is not owner",
			cmd: UpdateItemCommand{
				ItemID: itemID,
				UserID: otherUserID,
			},
			setupMock: func(repo *MockRepository) {
				repo.On("GetItemByID", mock.Anything, itemID).Return(&Item{
					ID:       itemID,
					SellerID: ownerID,
				}, nil)
			},
			wantErr: ErrUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockRepository)
			tt.setupMock(repo)

			service := NewService(repo)
			item, err := service.UpdateItem(context.Background(), tt.cmd)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, item)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, item)
				assert.Equal(t, tt.cmd.Title, item.Title)
				assert.Equal(t, tt.cmd.Description, item.Description)
			}

			repo.AssertExpectations(t)
		})
	}
}

func TestService_CancelItem(t *testing.T) {
	itemID := uuid.New()
	ownerID := uuid.New()
	otherUserID := uuid.New()

	tests := []struct {
		name      string
		cmd       CancelItemCommand
		setupMock func(*MockRepository)
		wantErr   error
	}{
		{
			name: "successfully cancels item with no bids",
			cmd: CancelItemCommand{
				ItemID: itemID,
				UserID: ownerID,
			},
			setupMock: func(repo *MockRepository) {
				repo.On("GetItemByID", mock.Anything, itemID).Return(&Item{
					ID:       itemID,
					SellerID: ownerID,
					Status:   ItemStatusActive,
				}, nil)
				repo.On("CountBidsByItemID", mock.Anything, itemID).Return(int64(0), nil)
				repo.On("UpdateStatus", mock.Anything, itemID, ItemStatusCancelled).Return(nil)
			},
			wantErr: nil,
		},
		{
			name: "fails when item not found",
			cmd: CancelItemCommand{
				ItemID: itemID,
				UserID: ownerID,
			},
			setupMock: func(repo *MockRepository) {
				repo.On("GetItemByID", mock.Anything, itemID).Return(nil, errors.New("not found"))
			},
			wantErr: ErrItemNotFound,
		},
		{
			name: "fails when user is not owner",
			cmd: CancelItemCommand{
				ItemID: itemID,
				UserID: otherUserID,
			},
			setupMock: func(repo *MockRepository) {
				repo.On("GetItemByID", mock.Anything, itemID).Return(&Item{
					ID:       itemID,
					SellerID: ownerID,
					Status:   ItemStatusActive,
				}, nil)
			},
			wantErr: ErrUnauthorized,
		},
		{
			name: "fails when item has bids",
			cmd: CancelItemCommand{
				ItemID: itemID,
				UserID: ownerID,
			},
			setupMock: func(repo *MockRepository) {
				repo.On("GetItemByID", mock.Anything, itemID).Return(&Item{
					ID:       itemID,
					SellerID: ownerID,
					Status:   ItemStatusActive,
				}, nil)
				repo.On("CountBidsByItemID", mock.Anything, itemID).Return(int64(5), nil)
			},
			wantErr: ErrCannotCancel,
		},
		{
			name: "fails when item is not active",
			cmd: CancelItemCommand{
				ItemID: itemID,
				UserID: ownerID,
			},
			setupMock: func(repo *MockRepository) {
				repo.On("GetItemByID", mock.Anything, itemID).Return(&Item{
					ID:       itemID,
					SellerID: ownerID,
					Status:   ItemStatusEnded,
				}, nil)
				repo.On("CountBidsByItemID", mock.Anything, itemID).Return(int64(0), nil)
			},
			wantErr: ErrCannotCancel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockRepository)
			tt.setupMock(repo)

			service := NewService(repo)
			item, err := service.CancelItem(context.Background(), tt.cmd)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, item)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, item)
				assert.Equal(t, ItemStatusCancelled, item.Status)
			}

			repo.AssertExpectations(t)
		})
	}
}

func TestService_ValidateSellerCannotBid(t *testing.T) {
	itemID := uuid.New()
	sellerID := uuid.New()
	otherUserID := uuid.New()

	tests := []struct {
		name      string
		itemID    uuid.UUID
		userID    uuid.UUID
		setupMock func(*MockRepository)
		wantErr   error
	}{
		{
			name:   "seller cannot bid on own item",
			itemID: itemID,
			userID: sellerID,
			setupMock: func(repo *MockRepository) {
				repo.On("GetItemByID", mock.Anything, itemID).Return(&Item{
					ID:       itemID,
					SellerID: sellerID,
				}, nil)
			},
			wantErr: ErrSellerCannotBid,
		},
		{
			name:   "other user can bid",
			itemID: itemID,
			userID: otherUserID,
			setupMock: func(repo *MockRepository) {
				repo.On("GetItemByID", mock.Anything, itemID).Return(&Item{
					ID:       itemID,
					SellerID: sellerID,
				}, nil)
			},
			wantErr: nil,
		},
		{
			name:   "fails when item not found",
			itemID: itemID,
			userID: otherUserID,
			setupMock: func(repo *MockRepository) {
				repo.On("GetItemByID", mock.Anything, itemID).Return(nil, errors.New("not found"))
			},
			wantErr: ErrItemNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := new(MockRepository)
			tt.setupMock(repo)

			service := NewService(repo)
			err := service.ValidateSellerCannotBid(context.Background(), tt.itemID, tt.userID)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}

			repo.AssertExpectations(t)
		})
	}
}
