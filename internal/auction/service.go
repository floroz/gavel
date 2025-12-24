package auction

import (
	"context"
	"fmt"
	"time"

	"github.com/floroz/auction-system/internal/pb"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Validation errors
var (
	ErrBidTooLow    = fmt.Errorf("bid amount must be higher than current highest bid")
	ErrAuctionEnded = fmt.Errorf("auction has ended")
)

// validateBidAmount checks if the bid amount is higher than the current highest bid
func validateBidAmount(bidAmount, currentHighest int64) error {
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
	txManager  TransactionManager
	bidRepo    BidRepository
	itemRepo   ItemRepository
	outboxRepo OutboxRepository
}

// NewAuctionService creates a new auction service
func NewAuctionService(
	txManager TransactionManager,
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
	defer tx.Rollback(ctx) // Rollback if commit is not called

	// Lock the item row to prevent race conditions
	// This ensures that only one transaction can modify this item at a time
	item, err := s.itemRepo.GetItemByIDForUpdate(ctx, tx, cmd.ItemID)
	if err != nil {
		return nil, fmt.Errorf("item not found: %w", err)
	}

	if err := validateBidAmount(cmd.Amount, item.CurrentHighestBid); err != nil {
		return nil, err
	}

	if err := validateAuctionNotEnded(item.EndAt); err != nil {
		return nil, err
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
	if err := s.bidRepo.SaveBid(ctx, tx, bid); err != nil {
		return nil, fmt.Errorf("failed to save bid: %w", err)
	}

	// Step 2: Update the item's highest bid
	if err := s.itemRepo.UpdateHighestBid(ctx, tx, cmd.ItemID, cmd.Amount); err != nil {
		return nil, fmt.Errorf("failed to update highest bid: %w", err)
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
	payload, err := proto.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	// Step 4: Save the event to the outbox (in the same transaction)
	outboxEvent := &OutboxEvent{
		ID:        uuid.New(),
		EventType: EventTypeBidPlaced,
		Payload:   payload,
		Status:    OutboxStatusPending,
		CreatedAt: time.Now(),
	}

	if err := s.outboxRepo.SaveEvent(ctx, tx, outboxEvent); err != nil {
		return nil, fmt.Errorf("failed to save outbox event: %w", err)
	}

	// Commit the transaction
	// If this succeeds, both the bid and the event are guaranteed to be saved
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return bid, nil
}
