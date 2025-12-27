package api

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

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
	// 1. Validation / Mapping
	itemID, err := uuid.Parse(req.Msg.ItemId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item_id"))
	}

	userID, err := uuid.Parse(req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid user_id"))
	}

	cmd := bids.PlaceBidCommand{
		ItemID: itemID,
		UserID: userID,
		Amount: req.Msg.Amount,
	}

	// 2. Execution
	bid, err := h.auctionService.PlaceBid(ctx, cmd)
	if err != nil {
		// In a real app, map domain errors to specific Connect codes (e.g., CodeNotFound, CodeFailedPrecondition)
		// For now, we return Internal for everything, but we should improve this.
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// 3. Response Mapping
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
