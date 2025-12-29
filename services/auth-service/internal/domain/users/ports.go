package users

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserRepository interface {
	CreateUser(ctx context.Context, tx pgx.Tx, user *User) error
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
}

type TokenRepository interface {
	CreateRefreshToken(ctx context.Context, tx pgx.Tx, token *RefreshToken) error
	GetRefreshToken(ctx context.Context, tokenHash []byte) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tx pgx.Tx, tokenHash []byte) error
	// RevokeAllUserTokens is useful for "logout from all devices" functionality
	RevokeAllUserTokens(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error
}

type OutboxRepository interface {
	CreateEvent(ctx context.Context, tx pgx.Tx, event *OutboxEvent) error
	GetPendingEvents(ctx context.Context, tx pgx.Tx, limit int) ([]*OutboxEvent, error)
	UpdateEventStatus(ctx context.Context, tx pgx.Tx, id uuid.UUID, status OutboxStatus) error
}

type EventPublisher interface {
	Publish(ctx context.Context, exchange, routingKey string, body []byte) error
}

type AuthService interface {
	Register(ctx context.Context, email, password, fullName, countryCode string) (*User, error)
	Login(ctx context.Context, email, password, userAgent, ip string) (accessToken, refreshToken string, err error)
	Refresh(ctx context.Context, refreshToken, userAgent, ip string) (newAccess, newRefresh string, err error)
	Logout(ctx context.Context, refreshToken string) error
	GetProfile(ctx context.Context, userID uuid.UUID) (*User, error)
}
