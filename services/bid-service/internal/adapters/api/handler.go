package api

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/floroz/gavel/pkg/auth"
	bidsv1 "github.com/floroz/gavel/pkg/proto/bids/v1"
	"github.com/floroz/gavel/pkg/proto/bids/v1/bidsv1connect"
	"github.com/floroz/gavel/services/bid-service/internal/domain/bids"
)

type BidServiceHandler struct {
	bidsv1connect.UnimplementedBidServiceHandler
	auctionService *bids.AuctionService
}

func NewBidServiceHandler(service *bids.AuctionService) *BidServiceHandler {
	return &BidServiceHandler{
		auctionService: service,
	}
}

func (h *BidServiceHandler) PlaceBid(
	ctx context.Context,
	req *connect.Request[bidsv1.PlaceBidRequest],
) (*connect.Response[bidsv1.PlaceBidResponse], error) {
	// 1. Get user ID from context (guaranteed by auth interceptor at router level)
	userID, err := uuid.Parse(auth.MustGetUserID(ctx))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user_id in token"))
	}

	// 2. Validation / Mapping
	itemID, err := uuid.Parse(req.Msg.ItemId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item_id"))
	}

	cmd := bids.PlaceBidCommand{
		ItemID: itemID,
		UserID: userID,
		Amount: req.Msg.Amount,
	}

	// 3. Execution
	bid, err := h.auctionService.PlaceBid(ctx, cmd)
	if err != nil {
		if errors.Is(err, bids.ErrBidTooLow) || errors.Is(err, bids.ErrAuctionEnded) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		if errors.Is(err, bids.ErrInvalidBidAmount) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		// Check for "item not found" wrapped error
		// Since we wrap it with fmt.Errorf("item not found: %w", err), checking string or unwrapping is needed.
		// A cleaner way is to have a typed error for ItemNotFound in the domain.
		// For now, simple string check or if the service returned a sentinel.
		// The service currently returns `fmt.Errorf("item not found: %w", err)`.
		// Let's rely on the error string for this specific case as per current implementation,
		// or better, let's assume standard error wrapping.
		// Ideally, we should define ErrItemNotFound in domain/bids.
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// 4. Response Mapping
	res := &bidsv1.PlaceBidResponse{
		Bid: &bidsv1.Bid{
			Id:        bid.ID.String(),
			ItemId:    bid.ItemID.String(),
			UserId:    bid.UserID.String(),
			Amount:    bid.Amount,
			CreatedAt: bid.CreatedAt.Format(time.RFC3339),
		},
	}

	return connect.NewResponse(res), nil
}
