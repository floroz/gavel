package tests

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/floroz/gavel/pkg/auth"
	"github.com/floroz/gavel/pkg/database"
	"github.com/floroz/gavel/pkg/proto/auth/v1/authv1connect"
	"github.com/floroz/gavel/services/auth-service/internal/adapters/api"
	infradb "github.com/floroz/gavel/services/auth-service/internal/adapters/database"
	"github.com/floroz/gavel/services/auth-service/internal/domain/users"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// setupAuthApp wires up the application for testing using a real database connection.
func setupAuthApp(t *testing.T, pool *pgxpool.Pool) (authv1connect.AuthServiceClient, *pgxpool.Pool) {
	// 1. Initialize Repositories
	txManager := database.NewPostgresTransactionManager(pool, 5*time.Second)
	userRepo := infradb.NewPostgresUserRepository(pool)
	tokenRepo := infradb.NewPostgresTokenRepository(pool)

	// 2. Initialize Dependencies
	// Generate ephemeral RSA keys for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	})

	pubBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	signer, err := auth.NewSigner(privPEM, pubPEM)
	require.NoError(t, err)

	// 3. Initialize Service
	authService := users.NewService(userRepo, tokenRepo, signer, txManager)

	// 4. Initialize API Handler
	authHandler := api.NewAuthServiceHandler(authService)
	path, handler := authv1connect.NewAuthServiceHandler(authHandler)

	// 5. Create Test Server
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	// 6. Create Client
	client := authv1connect.NewAuthServiceClient(
		server.Client(),
		server.URL,
	)

	return client, pool
}

// verifyUserExists checks if a user exists in the database.
func verifyUserExists(t *testing.T, pool *pgxpool.Pool, email string) *users.User {
	t.Helper()
	var user users.User
	query := `SELECT id, email, full_name, country_code FROM users WHERE email = $1`
	row := pool.QueryRow(context.Background(), query, email)
	err := row.Scan(&user.ID, &user.Email, &user.FullName, &user.CountryCode)
	if err != nil {
		return nil
	}
	return &user
}

// verifyTokenExists checks if a refresh token exists and matches the user.
func verifyTokenExists(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) bool {
	t.Helper()
	var count int
	query := `SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1 AND revoked = false`
	row := pool.QueryRow(context.Background(), query, userID)
	_ = row.Scan(&count)
	return count > 0
}
