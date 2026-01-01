package users

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/floroz/gavel/pkg/auth"
	"github.com/floroz/gavel/pkg/database"
	"github.com/floroz/gavel/pkg/events"
	pb "github.com/floroz/gavel/pkg/proto"
)

var (
	ErrUserAlreadyExists  = errors.New("user with this email already exists")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInvalidToken       = errors.New("invalid or expired refresh token")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidInput       = errors.New("invalid input")
)

type Service struct {
	userRepo   UserRepository
	tokenRepo  TokenRepository
	outboxRepo OutboxRepository
	signer     *auth.Signer
	txManager  database.TransactionManager
}

func NewService(
	userRepo UserRepository,
	tokenRepo TokenRepository,
	outboxRepo OutboxRepository,
	signer *auth.Signer,
	txManager database.TransactionManager,
) *Service {
	return &Service{
		userRepo:   userRepo,
		tokenRepo:  tokenRepo,
		outboxRepo: outboxRepo,
		signer:     signer,
		txManager:  txManager,
	}
}

func (s *Service) Register(ctx context.Context, email, password, fullName, phoneNumber, countryCode string) (*User, error) {
	if err := validateUser(email, password, fullName, phoneNumber, countryCode); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	// Check if user already exists
	existing, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}
	if existing != nil {
		return nil, ErrUserAlreadyExists
	}

	// Hash password
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create User
	now := time.Now()
	user := &User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: hash,
		FullName:     fullName,
		PhoneNumber:  phoneNumber,
		CountryCode:  countryCode,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Transaction: Save User and Outbox Event
	tx, err := s.txManager.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.userRepo.CreateUser(ctx, tx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create Outbox Event
	event := &pb.UserCreated{
		UserId:      user.ID.String(),
		Email:       user.Email,
		FullName:    user.FullName,
		CountryCode: user.CountryCode,
		CreatedAt:   timestamppb.New(user.CreatedAt),
	}
	payload, err := proto.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}

	outboxEvent := &events.OutboxEvent{
		ID:        uuid.New(),
		EventType: "user.created",
		Payload:   payload,
		Status:    events.OutboxStatusPending,
		CreatedAt: now,
	}

	if err := s.outboxRepo.CreateEvent(ctx, tx, outboxEvent); err != nil {
		return nil, fmt.Errorf("failed to create outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return user, nil
}

func (s *Service) Login(ctx context.Context, email, password, userAgent, ip string) (string, string, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, email)
	if err != nil {
		return "", "", fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return "", "", ErrInvalidCredentials
	}

	// Verify password
	valid, err := auth.VerifyPassword(user.PasswordHash, password)
	if err != nil {
		return "", "", fmt.Errorf("failed to verify password: %w", err)
	}
	if !valid {
		return "", "", ErrInvalidCredentials
	}

	return s.generateAndSaveTokens(ctx, user, userAgent, ip)
}

func (s *Service) Refresh(ctx context.Context, refreshToken, userAgent, ip string) (string, string, error) {
	// Hash the incoming token to look it up
	tokenHash := hashToken(refreshToken)

	// Get stored token
	storedToken, err := s.tokenRepo.GetRefreshToken(ctx, tokenHash)
	if err != nil {
		return "", "", fmt.Errorf("failed to get refresh token: %w", err)
	}
	if storedToken == nil {
		return "", "", ErrInvalidToken
	}

	// Check validity
	if storedToken.Revoked {
		// Potential reuse attack!
		// We should probably revoke all user tokens here for security.
		// For now, just return error.
		return "", "", ErrInvalidToken
	}
	if time.Now().After(storedToken.ExpiresAt) {
		return "", "", ErrInvalidToken
	}

	// Get User
	user, err := s.userRepo.GetUserByID(ctx, storedToken.UserID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return "", "", ErrUserNotFound
	}

	// Rotate tokens: Revoke old one, issue new ones
	tx, err := s.txManager.BeginTx(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Revoke old token
	if err := s.tokenRepo.RevokeRefreshToken(ctx, tx, tokenHash); err != nil {
		return "", "", fmt.Errorf("failed to revoke token: %w", err)
	}

	// Generate and save new tokens (inside the same transaction)
	// We duplicate generateAndSaveTokens logic slightly here to use the existing tx
	tokenPair, err := s.signer.GenerateTokens(user.ID, user.Email, user.FullName, nil) // Permissions empty for now
	if err != nil {
		return "", "", fmt.Errorf("failed to generate tokens: %w", err)
	}

	newTokenHash := hashToken(tokenPair.RefreshToken)
	newStoredToken := &RefreshToken{
		TokenHash: newTokenHash,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // 7 days
		Revoked:   false,
		CreatedAt: time.Now(),
		UserAgent: userAgent,
		IPAddress: ip,
	}

	if err := s.tokenRepo.CreateRefreshToken(ctx, tx, newStoredToken); err != nil {
		return "", "", fmt.Errorf("failed to save refresh token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return tokenPair.AccessToken, tokenPair.RefreshToken, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	tokenHash := hashToken(refreshToken)

	tx, err := s.txManager.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.tokenRepo.RevokeRefreshToken(ctx, tx, tokenHash); err != nil {
		return fmt.Errorf("failed to revoke token: %w", err)
	}

	return tx.Commit(ctx)
}

func (s *Service) GetProfile(ctx context.Context, userID uuid.UUID) (*User, error) {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// Helpers

func (s *Service) generateAndSaveTokens(ctx context.Context, user *User, userAgent, ip string) (string, string, error) {
	// Generate Tokens
	tokenPair, err := s.signer.GenerateTokens(user.ID, user.Email, user.FullName, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate tokens: %w", err)
	}

	// Save Refresh Token
	tokenHash := hashToken(tokenPair.RefreshToken)
	refreshToken := &RefreshToken{
		TokenHash: tokenHash,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // 7 days refresh token validity
		Revoked:   false,
		CreatedAt: time.Now(),
		UserAgent: userAgent,
		IPAddress: ip,
	}

	tx, err := s.txManager.BeginTx(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := s.tokenRepo.CreateRefreshToken(ctx, tx, refreshToken); err != nil {
		return "", "", fmt.Errorf("failed to save refresh token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", "", fmt.Errorf("failed to commit transaction: %w", err)
	}

	return tokenPair.AccessToken, tokenPair.RefreshToken, nil
}

func hashToken(token string) []byte {
	hash := sha256.Sum256([]byte(token))
	return hash[:]
}

func validateUser(email, password, fullName, phoneNumber, countryCode string) error {
	if !strings.Contains(email, "@") || len(email) < 3 {
		return errors.New("invalid email format")
	}
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	if strings.TrimSpace(fullName) == "" {
		return errors.New("full name cannot be empty")
	}
	if strings.TrimSpace(phoneNumber) == "" {
		return errors.New("phone number cannot be empty")
	}
	if len(countryCode) != 2 || countryCode != strings.ToUpper(countryCode) {
		return errors.New("country code must be 2 uppercase letters (ISO 3166-1 alpha-2)")
	}
	for _, r := range countryCode {
		if r < 'A' || r > 'Z' {
			return errors.New("country code must contain only letters")
		}
	}
	return nil
}
