package userstats

import (
	"context"
	"fmt"

	"github.com/floroz/gavel/pkg/database"
)

type Service struct {
	repo      Repository
	txManager database.TransactionManager
}

func NewService(repo Repository, txManager database.TransactionManager) *Service {
	return &Service{
		repo:      repo,
		txManager: txManager,
	}
}

func (s *Service) ProcessBidPlaced(ctx context.Context, event BidPlacedEvent) error {
	// 1. Start Transaction
	tx, err := s.txManager.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	// 2. Check Idempotency (Has this event been processed?)
	isProcessed, err := s.repo.IsEventProcessed(ctx, tx, event.EventID)
	if err != nil {
		return fmt.Errorf("failed to check idempotency: %w", err)
	}
	if isProcessed {
		// Already processed, acknowledge and return (Idempotent Success)
		return nil
	}

	// 3. Update User Stats (Increment/Upsert)
	// We no longer construct a struct with "1". We explicitly call Increment.
	if err := s.repo.IncrementUserStats(ctx, tx, event.UserID, event.Amount, event.Timestamp); err != nil {
		return fmt.Errorf("failed to increment user stats: %w", err)
	}

	// 4. Mark Event as Processed
	if err := s.repo.MarkEventProcessed(ctx, tx, event.EventID); err != nil {
		return fmt.Errorf("failed to mark event as processed: %w", err)
	}

	// 5. Commit
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
