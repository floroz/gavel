package api

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	userstatsv1 "github.com/floroz/gavel/pkg/proto/userstats/v1"
	"github.com/floroz/gavel/pkg/proto/userstats/v1/userstatsv1connect"
	"github.com/floroz/gavel/services/user-stats-service/internal/domain/userstats"
)

type UserStatsServiceHandler struct {
	userstatsv1connect.UnimplementedUserStatsServiceHandler
	service *userstats.Service
}

func NewUserStatsServiceHandler(service *userstats.Service) *UserStatsServiceHandler {
	return &UserStatsServiceHandler{
		service: service,
	}
}

func (h *UserStatsServiceHandler) GetUserStats(
	ctx context.Context,
	req *connect.Request[userstatsv1.GetUserStatsRequest],
) (*connect.Response[userstatsv1.UserStatsResponse], error) {
	userID, err := uuid.Parse(req.Msg.UserId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid user_id"))
	}

	stats, err := h.service.GetUserStats(ctx, userID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	if stats == nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user stats not found"))
	}

	res := &userstatsv1.UserStatsResponse{
		Stats: &userstatsv1.UserStats{
			UserId:        stats.UserID.String(),
			TotalBids:     stats.TotalBidsPlaced,
			TotalAmount:   stats.TotalAmountBid,
			LastUpdatedAt: stats.LastBidAt.Format(time.RFC3339),
		},
	}

	return connect.NewResponse(res), nil
}
