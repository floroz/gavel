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
	"github.com/floroz/gavel/services/bid-service/internal/domain/items"
)

type BidServiceHandler struct {
	bidsv1connect.UnimplementedBidServiceHandler
	auctionService *bids.AuctionService
	itemService    *items.Service
	bidRepo        bids.BidRepository
}

func NewBidServiceHandler(auctionService *bids.AuctionService, itemService *items.Service, bidRepo bids.BidRepository) *BidServiceHandler {
	return &BidServiceHandler{
		auctionService: auctionService,
		itemService:    itemService,
		bidRepo:        bidRepo,
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
		if errors.Is(err, bids.ErrSellerCannotBid) {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
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

// CreateItem creates a new auction item
func (h *BidServiceHandler) CreateItem(
	ctx context.Context,
	req *connect.Request[bidsv1.CreateItemRequest],
) (*connect.Response[bidsv1.CreateItemResponse], error) {
	// Get user ID from context (auth required)
	userID, err := uuid.Parse(auth.MustGetUserID(ctx))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user_id in token"))
	}

	// Parse end time
	endAt, err := time.Parse(time.RFC3339, req.Msg.EndAt)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid end_at format"))
	}

	// Create command
	cmd := items.CreateItemCommand{
		Title:       req.Msg.Title,
		Description: req.Msg.Description,
		StartPrice:  req.Msg.StartPrice,
		EndAt:       endAt,
		Images:      req.Msg.Images,
		Category:    req.Msg.Category,
		SellerID:    userID,
	}

	// Execute
	item, err := h.itemService.CreateItem(ctx, cmd)
	if err != nil {
		if errors.Is(err, items.ErrInvalidStartPrice) || errors.Is(err, items.ErrInvalidEndTime) {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Map to proto
	res := &bidsv1.CreateItemResponse{
		Item: mapItemToProto(item),
	}

	return connect.NewResponse(res), nil
}

// GetItem retrieves an item by ID
func (h *BidServiceHandler) GetItem(
	ctx context.Context,
	req *connect.Request[bidsv1.GetItemRequest],
) (*connect.Response[bidsv1.GetItemResponse], error) {
	// Parse item ID
	itemID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}

	// Execute
	item, err := h.itemService.GetItem(ctx, itemID)
	if err != nil {
		if errors.Is(err, items.ErrItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Map to proto
	res := &bidsv1.GetItemResponse{
		Item: mapItemToProto(item),
	}

	return connect.NewResponse(res), nil
}

// ListItems retrieves active items with pagination
func (h *BidServiceHandler) ListItems(
	ctx context.Context,
	req *connect.Request[bidsv1.ListItemsRequest],
) (*connect.Response[bidsv1.ListItemsResponse], error) {
	// Use page_size for limit (default 20)
	// Note: page_token is not implemented yet - would require cursor-based pagination
	limit := int(req.Msg.PageSize)
	if limit <= 0 {
		limit = 20
	}

	// Execute (using offset 0 for now - proper pagination would decode page_token)
	itemList, err := h.itemService.ListItems(ctx, items.ListItemsQuery{
		Limit:  limit,
		Offset: 0,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Map to proto
	protoItems := make([]*bidsv1.Item, len(itemList))
	for i, item := range itemList {
		protoItems[i] = mapItemToProto(item)
	}

	res := &bidsv1.ListItemsResponse{
		Items: protoItems,
		// TODO: Implement next_page_token for cursor-based pagination
	}

	return connect.NewResponse(res), nil
}

// ListSellerItems retrieves all items for the authenticated seller
func (h *BidServiceHandler) ListSellerItems(
	ctx context.Context,
	req *connect.Request[bidsv1.ListSellerItemsRequest],
) (*connect.Response[bidsv1.ListSellerItemsResponse], error) {
	// Get user ID from context (auth required)
	userID, err := uuid.Parse(auth.MustGetUserID(ctx))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user_id in token"))
	}

	// Use page_size for limit (default 20)
	limit := int(req.Msg.PageSize)
	if limit <= 0 {
		limit = 20
	}

	// Execute (using offset 0 for now - proper pagination would decode page_token)
	itemList, err := h.itemService.ListSellerItems(ctx, items.ListSellerItemsQuery{
		SellerID: userID,
		Limit:    limit,
		Offset:   0,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Map to proto
	protoItems := make([]*bidsv1.Item, len(itemList))
	for i, item := range itemList {
		protoItems[i] = mapItemToProto(item)
	}

	res := &bidsv1.ListSellerItemsResponse{
		Items: protoItems,
		// TODO: Implement next_page_token for cursor-based pagination
	}

	return connect.NewResponse(res), nil
}

// UpdateItem updates an item's editable fields
func (h *BidServiceHandler) UpdateItem(
	ctx context.Context,
	req *connect.Request[bidsv1.UpdateItemRequest],
) (*connect.Response[bidsv1.UpdateItemResponse], error) {
	// Get user ID from context (auth required)
	userID, err := uuid.Parse(auth.MustGetUserID(ctx))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user_id in token"))
	}

	// Parse item ID
	itemID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}

	// First get the existing item to preserve fields that aren't being updated
	existingItem, err := h.itemService.GetItem(ctx, itemID)
	if err != nil {
		if errors.Is(err, items.ErrItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Create command with optional fields
	title := existingItem.Title
	if req.Msg.Title != nil {
		title = *req.Msg.Title
	}

	description := existingItem.Description
	if req.Msg.Description != nil {
		description = *req.Msg.Description
	}

	category := existingItem.Category
	if req.Msg.Category != nil {
		category = *req.Msg.Category
	}

	images := existingItem.Images
	if len(req.Msg.Images) > 0 {
		images = req.Msg.Images
	}

	cmd := items.UpdateItemCommand{
		ItemID:      itemID,
		UserID:      userID,
		Title:       title,
		Description: description,
		Images:      images,
		Category:    category,
	}

	// Execute
	item, err := h.itemService.UpdateItem(ctx, cmd)
	if err != nil {
		if errors.Is(err, items.ErrItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if errors.Is(err, items.ErrUnauthorized) {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Map to proto
	res := &bidsv1.UpdateItemResponse{
		Item: mapItemToProto(item),
	}

	return connect.NewResponse(res), nil
}

// CancelItem cancels an auction item
func (h *BidServiceHandler) CancelItem(
	ctx context.Context,
	req *connect.Request[bidsv1.CancelItemRequest],
) (*connect.Response[bidsv1.CancelItemResponse], error) {
	// Get user ID from context (auth required)
	userID, err := uuid.Parse(auth.MustGetUserID(ctx))
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, errors.New("invalid user_id in token"))
	}

	// Parse item ID
	itemID, err := uuid.Parse(req.Msg.Id)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid id"))
	}

	// Create command
	cmd := items.CancelItemCommand{
		ItemID: itemID,
		UserID: userID,
	}

	// Execute
	item, err := h.itemService.CancelItem(ctx, cmd)
	if err != nil {
		if errors.Is(err, items.ErrItemNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if errors.Is(err, items.ErrUnauthorized) {
			return nil, connect.NewError(connect.CodePermissionDenied, err)
		}
		if errors.Is(err, items.ErrCannotCancel) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Map to proto and return
	res := &bidsv1.CancelItemResponse{
		Item: mapItemToProto(item),
	}
	return connect.NewResponse(res), nil
}

// GetItemBids retrieves all bids for an item
func (h *BidServiceHandler) GetItemBids(
	ctx context.Context,
	req *connect.Request[bidsv1.GetItemBidsRequest],
) (*connect.Response[bidsv1.GetItemBidsResponse], error) {
	// Parse item ID
	itemID, err := uuid.Parse(req.Msg.ItemId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid item_id"))
	}

	// Execute
	bidList, err := h.bidRepo.GetBidsByItemID(ctx, itemID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Map to proto
	protoBids := make([]*bidsv1.Bid, len(bidList))
	for i, bid := range bidList {
		protoBids[i] = &bidsv1.Bid{
			Id:        bid.ID.String(),
			ItemId:    bid.ItemID.String(),
			UserId:    bid.UserID.String(),
			Amount:    bid.Amount,
			CreatedAt: bid.CreatedAt.Format(time.RFC3339),
		}
	}

	res := &bidsv1.GetItemBidsResponse{
		Bids: protoBids,
	}

	return connect.NewResponse(res), nil
}

// mapItemToProto converts a domain Item to a proto Item
func mapItemToProto(item *items.Item) *bidsv1.Item {
	// Map status
	var protoStatus bidsv1.ItemStatus
	switch item.Status {
	case items.ItemStatusActive:
		protoStatus = bidsv1.ItemStatus_ITEM_STATUS_ACTIVE
	case items.ItemStatusEnded:
		protoStatus = bidsv1.ItemStatus_ITEM_STATUS_ENDED
	case items.ItemStatusCancelled:
		protoStatus = bidsv1.ItemStatus_ITEM_STATUS_CANCELLED
	default:
		protoStatus = bidsv1.ItemStatus_ITEM_STATUS_UNSPECIFIED
	}

	return &bidsv1.Item{
		Id:                item.ID.String(),
		Title:             item.Title,
		Description:       item.Description,
		StartPrice:        item.StartPrice,
		CurrentHighestBid: item.CurrentHighestBid,
		EndAt:             item.EndAt.Format(time.RFC3339),
		CreatedAt:         item.CreatedAt.Format(time.RFC3339),
		UpdatedAt:         item.UpdatedAt.Format(time.RFC3339),
		Images:            item.Images,
		Category:          item.Category,
		SellerId:          item.SellerID.String(),
		Status:            protoStatus,
	}
}
