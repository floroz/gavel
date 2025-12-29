package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/floroz/gavel/services/auth-service/internal/domain/users"
)

// PostgresUserRepository implements users.UserRepository
type PostgresUserRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresUserRepository(pool *pgxpool.Pool) *PostgresUserRepository {
	return &PostgresUserRepository{pool: pool}
}

func (r *PostgresUserRepository) CreateUser(ctx context.Context, tx pgx.Tx, user *users.User) error {
	query := `
		INSERT INTO users (id, email, password_hash, full_name, avatar_url, phone_number, country_code, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := tx.Exec(ctx, query,
		user.ID,
		user.Email,
		user.PasswordHash,
		user.FullName,
		user.AvatarURL,
		user.PhoneNumber,
		user.CountryCode,
		user.CreatedAt,
		user.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

func (r *PostgresUserRepository) GetUserByID(ctx context.Context, id uuid.UUID) (*users.User, error) {
	query := `
		SELECT id, email, password_hash, full_name, avatar_url, phone_number, country_code, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var user users.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FullName,
		&user.AvatarURL,
		&user.PhoneNumber,
		&user.CountryCode,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Return nil if not found, let service handle it
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return &user, nil
}

func (r *PostgresUserRepository) GetUserByEmail(ctx context.Context, email string) (*users.User, error) {
	query := `
		SELECT id, email, password_hash, full_name, avatar_url, phone_number, country_code, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var user users.User
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.FullName,
		&user.AvatarURL,
		&user.PhoneNumber,
		&user.CountryCode,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}

// PostgresTokenRepository implements users.TokenRepository
type PostgresTokenRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresTokenRepository(pool *pgxpool.Pool) *PostgresTokenRepository {
	return &PostgresTokenRepository{pool: pool}
}

func (r *PostgresTokenRepository) CreateRefreshToken(ctx context.Context, tx pgx.Tx, token *users.RefreshToken) error {
	query := `
		INSERT INTO refresh_tokens (token_hash, user_id, expires_at, revoked, created_at, user_agent, ip_address)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := tx.Exec(ctx, query,
		token.TokenHash,
		token.UserID,
		token.ExpiresAt,
		token.Revoked,
		token.CreatedAt,
		token.UserAgent,
		token.IPAddress,
	)
	if err != nil {
		return fmt.Errorf("failed to create refresh token: %w", err)
	}
	return nil
}

func (r *PostgresTokenRepository) GetRefreshToken(ctx context.Context, tokenHash []byte) (*users.RefreshToken, error) {
	query := `
		SELECT token_hash, user_id, expires_at, revoked, created_at, user_agent, ip_address
		FROM refresh_tokens
		WHERE token_hash = $1
	`
	var token users.RefreshToken
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&token.TokenHash,
		&token.UserID,
		&token.ExpiresAt,
		&token.Revoked,
		&token.CreatedAt,
		&token.UserAgent,
		&token.IPAddress,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}
	return &token, nil
}

func (r *PostgresTokenRepository) RevokeRefreshToken(ctx context.Context, tx pgx.Tx, tokenHash []byte) error {
	query := `UPDATE refresh_tokens SET revoked = true WHERE token_hash = $1`
	_, err := tx.Exec(ctx, query, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

func (r *PostgresTokenRepository) RevokeAllUserTokens(ctx context.Context, tx pgx.Tx, userID uuid.UUID) error {
	query := `UPDATE refresh_tokens SET revoked = true WHERE user_id = $1`
	_, err := tx.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke all user tokens: %w", err)
	}
	return nil
}
