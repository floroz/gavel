package items

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Service errors
var (
	ErrInvalidStartPrice = fmt.Errorf("start price must be greater than 0")
	ErrInvalidEndTime    = fmt.Errorf("end time must be in the future")
	ErrItemNotFound      = fmt.Errorf("item not found")
	ErrUnauthorized      = fmt.Errorf("unauthorized: only the owner can perform this action")
	ErrCannotCancel      = fmt.Errorf("cannot cancel item: item has bids or is not active")
	ErrItemNotActive     = fmt.Errorf("item is not active")
	ErrSellerCannotBid   = fmt.Errorf("seller cannot bid on their own item")
)

// CreateItemCommand represents the command to create a new item
type CreateItemCommand struct {
	Title       string
	Description string
	StartPrice  int64
	EndAt       time.Time
	Images      []string
	Category    string
	SellerID    uuid.UUID
}

// UpdateItemCommand represents the command to update an item
type UpdateItemCommand struct {
	ItemID      uuid.UUID
	UserID      uuid.UUID
	Title       string
	Description string
	Images      []string
	Category    string
}

// CancelItemCommand represents the command to cancel an item
type CancelItemCommand struct {
	ItemID uuid.UUID
	UserID uuid.UUID
}

// ListItemsQuery represents pagination parameters for listing items
type ListItemsQuery struct {
	Limit  int
	Offset int
}

// ListSellerItemsQuery represents pagination parameters for listing seller's items
type ListSellerItemsQuery struct {
	SellerID uuid.UUID
	Limit    int
	Offset   int
}

// Service implements the core business logic for items
type Service struct {
	repo Repository
}

// NewService creates a new item service
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateItem creates a new auction item
func (s *Service) CreateItem(ctx context.Context, cmd CreateItemCommand) (*Item, error) {
	// Validate start price
	if cmd.StartPrice <= 0 {
		return nil, ErrInvalidStartPrice
	}

	// Validate end time
	if !cmd.EndAt.After(time.Now()) {
		return nil, ErrInvalidEndTime
	}

	// Create item
	item := &Item{
		ID:                uuid.New(),
		Title:             cmd.Title,
		Description:       cmd.Description,
		StartPrice:        cmd.StartPrice,
		CurrentHighestBid: 0,
		EndAt:             cmd.EndAt,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		Images:            cmd.Images,
		Category:          cmd.Category,
		SellerID:          cmd.SellerID,
		Status:            ItemStatusActive,
	}

	if err := s.repo.CreateItem(ctx, item); err != nil {
		return nil, fmt.Errorf("failed to create item: %w", err)
	}

	return item, nil
}

// GetItem retrieves an item by ID
func (s *Service) GetItem(ctx context.Context, itemID uuid.UUID) (*Item, error) {
	item, err := s.repo.GetItemByID(ctx, itemID)
	if err != nil {
		return nil, ErrItemNotFound
	}
	return item, nil
}

// ListItems retrieves active items with pagination
func (s *Service) ListItems(ctx context.Context, query ListItemsQuery) ([]*Item, error) {
	items, err := s.repo.ListActiveItems(ctx, query.Limit, query.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list items: %w", err)
	}
	return items, nil
}

// ListSellerItems retrieves all items for a specific seller
func (s *Service) ListSellerItems(ctx context.Context, query ListSellerItemsQuery) ([]*Item, error) {
	items, err := s.repo.ListItemsBySellerID(ctx, query.SellerID, query.Limit, query.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list seller items: %w", err)
	}
	return items, nil
}

// UpdateItem updates an item's editable fields
func (s *Service) UpdateItem(ctx context.Context, cmd UpdateItemCommand) (*Item, error) {
	// Get the item
	item, err := s.repo.GetItemByID(ctx, cmd.ItemID)
	if err != nil {
		return nil, ErrItemNotFound
	}

	// Check ownership
	if !item.IsOwnedBy(cmd.UserID) {
		return nil, ErrUnauthorized
	}

	// Update editable fields
	item.Title = cmd.Title
	item.Description = cmd.Description
	item.Images = cmd.Images
	item.Category = cmd.Category
	item.UpdatedAt = time.Now()

	if err := s.repo.UpdateItem(ctx, item); err != nil {
		return nil, fmt.Errorf("failed to update item: %w", err)
	}

	return item, nil
}

// CancelItem cancels an auction item
func (s *Service) CancelItem(ctx context.Context, cmd CancelItemCommand) (*Item, error) {
	// Get the item
	item, err := s.repo.GetItemByID(ctx, cmd.ItemID)
	if err != nil {
		return nil, ErrItemNotFound
	}

	// Check ownership
	if !item.IsOwnedBy(cmd.UserID) {
		return nil, ErrUnauthorized
	}

	// Check if item has bids
	bidCount, err := s.repo.CountBidsByItemID(ctx, cmd.ItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to check bids: %w", err)
	}

	hasBids := bidCount > 0

	// Check if item can be cancelled
	if !item.CanBeCancelled(hasBids) {
		return nil, ErrCannotCancel
	}

	// Update status to cancelled
	if err := s.repo.UpdateStatus(ctx, cmd.ItemID, ItemStatusCancelled); err != nil {
		return nil, fmt.Errorf("failed to cancel item: %w", err)
	}

	// Update item status and return
	item.Status = ItemStatusCancelled
	return item, nil
}

// ValidateSellerCannotBid checks if a user is trying to bid on their own item
func (s *Service) ValidateSellerCannotBid(ctx context.Context, itemID, userID uuid.UUID) error {
	item, err := s.repo.GetItemByID(ctx, itemID)
	if err != nil {
		return ErrItemNotFound
	}

	if item.SellerID == userID {
		return ErrSellerCannotBid
	}

	return nil
}
