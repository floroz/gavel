package bids

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/floroz/gavel/pkg/database"
	"github.com/floroz/gavel/pkg/events"
	pb "github.com/floroz/gavel/pkg/proto"
)

type PlaceBidCommand struct {
	ItemID uuid.UUID
	UserID uuid.UUID
	Amount int64
}

// Validation errors
var (
	ErrBidTooLow        = fmt.Errorf("bid amount must be higher than current highest bid")
	ErrAuctionEnded     = fmt.Errorf("auction has ended")
	ErrInvalidBidAmount = fmt.Errorf("bid amount must be positive")
	ErrSellerCannotBid  = fmt.Errorf("seller cannot bid on their own item")
)

// validateBidAmount checks if the bid amount is higher than the current highest bid
func validateBidAmount(bidAmount, currentHighest int64) error {
	if bidAmount <= 0 {
		return ErrInvalidBidAmount
	}
	if bidAmount <= currentHighest {
		return ErrBidTooLow
	}
	return nil
}

// validateAuctionNotEnded checks if the auction has not ended
func validateAuctionNotEnded(endAt time.Time) error {
	if time.Now().After(endAt) {
		return ErrAuctionEnded
	}
	return nil
}

// AuctionService implements the core business logic
type AuctionService struct {
	txManager  database.TransactionManager
	bidRepo    BidRepository
	itemRepo   ItemRepository
	outboxRepo OutboxRepository
}

// NewAuctionService creates a new auction service
func NewAuctionService(
	txManager database.TransactionManager,
	bidRepo BidRepository,
	itemRepo ItemRepository,
	outboxRepo OutboxRepository,
) *AuctionService {
	return &AuctionService{
		txManager:  txManager,
		bidRepo:    bidRepo,
		itemRepo:   itemRepo,
		outboxRepo: outboxRepo,
	}
}

// PlaceBid implements the transactional outbox pattern
// It saves the bid and the event in the same database transaction
func (s *AuctionService) PlaceBid(ctx context.Context, cmd PlaceBidCommand) (*Bid, error) {
	// Start transaction
	tx, err := s.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx) // Rollback if commit is not called
	}()

	// Lock the item row to prevent race conditions
	// This ensures that only one transaction can modify this item at a time
	item, err := s.itemRepo.GetItemByIDForUpdate(ctx, tx, cmd.ItemID)
	if err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	// Validate seller cannot bid on own item
	if item.SellerID == cmd.UserID {
		return nil, ErrSellerCannotBid
	}

	if valErr := validateBidAmount(cmd.Amount, item.CurrentHighestBid); valErr != nil {
		return nil, valErr
	}

	if valErr := validateAuctionNotEnded(item.EndAt); valErr != nil {
		return nil, valErr
	}

	// Create the bid
	bid := &Bid{
		ID:        uuid.New(),
		ItemID:    cmd.ItemID,
		UserID:    cmd.UserID,
		Amount:    cmd.Amount,
		CreatedAt: time.Now(),
	}

	// Step 1: Save the bid
	if saveErr := s.bidRepo.SaveBid(ctx, tx, bid); saveErr != nil {
		return nil, fmt.Errorf("failed to save bid: %w", saveErr)
	}

	// Step 2: Update the item's highest bid
	if updateErr := s.itemRepo.UpdateHighestBid(ctx, tx, cmd.ItemID, cmd.Amount); updateErr != nil {
		return nil, fmt.Errorf("failed to update highest bid: %w", updateErr)
	}

	// Step 3: Create the event (protobuf message)
	event := &pb.BidPlaced{
		BidId:     bid.ID.String(),
		ItemId:    bid.ItemID.String(),
		UserId:    bid.UserID.String(),
		Amount:    bid.Amount,
		Timestamp: timestamppb.New(bid.CreatedAt),
	}

	// Marshal the protobuf message
	payload, marshalErr := proto.Marshal(event)
	if marshalErr != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", marshalErr)
	}

	// Step 4: Save the event to the outbox (in the same transaction)
	outboxEvent := &events.OutboxEvent{
		ID:        uuid.New(),
		EventType: "bid.placed",
		Payload:   payload,
		Status:    events.OutboxStatusPending,
		CreatedAt: time.Now(),
	}

	if saveErr := s.outboxRepo.SaveEvent(ctx, tx, outboxEvent); saveErr != nil {
		return nil, fmt.Errorf("failed to save outbox event: %w", saveErr)
	}

	// Commit the transaction
	// If this succeeds, both the bid and the event are guaranteed to be saved
	if commitErr := tx.Commit(ctx); commitErr != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", commitErr)
	}

	return bid, nil
}
