package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/floroz/gavel/pkg/auth"
	pkgdb "github.com/floroz/gavel/pkg/database"
	"github.com/floroz/gavel/pkg/proto/userstats/v1/userstatsv1connect"
	"github.com/floroz/gavel/services/user-stats-service/internal/adapters/api"
	"github.com/floroz/gavel/services/user-stats-service/internal/adapters/database"
	"github.com/floroz/gavel/services/user-stats-service/internal/domain/userstats"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Load environment variables (local overrides .env)
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load()

	ctx := context.Background()

	// 1. Load JWT Public Key for token validation
	publicKeyPath := os.Getenv("JWT_PUBLIC_KEY_PATH")
	if publicKeyPath == "" {
		logger.Error("JWT_PUBLIC_KEY_PATH is not set")
		os.Exit(1)
	}

	publicKeyPEM, err := os.ReadFile(publicKeyPath)
	if err != nil {
		logger.Error("Failed to read public key", "path", publicKeyPath, "error", err)
		os.Exit(1)
	}

	issuer := os.Getenv("JWT_ISSUER")
	if issuer == "" {
		logger.Error("JWT_ISSUER is not set")
		os.Exit(1)
	}

	// Create signer with only public key (for validation only)
	signer, err := auth.NewSignerFromPublicKey(publicKeyPEM, issuer)
	if err != nil {
		logger.Error("Failed to create signer", "error", err)
		os.Exit(1)
	}
	logger.Info("JWT public key loaded", "path", publicKeyPath)

	// 2. Initialize Postgres Connection Pool
	dbURL := os.Getenv("USER_STATS_DB_URL")
	if dbURL == "" {
		logger.Error("USER_STATS_DB_URL is not set")
		os.Exit(1)
	}
	dbConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		logger.Error("Unable to parse database config", "error", err)
		os.Exit(1)
	}
	pool, err := pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		logger.Error("Unable to create connection pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err = pool.Ping(ctx); err != nil {
		logger.Error("Unable to ping database", "error", err)
		os.Exit(1)
	}
	logger.Info("Postgres Connected")

	// 2. Initialize Dependencies
	txManager := pkgdb.NewPostgresTransactionManager(pool, 5*time.Second)
	statsRepo := database.NewUserStatsRepository(pool)
	statsService := userstats.NewService(statsRepo, txManager)

	// 4. Initialize API Handler with auth interceptor
	statsHandler := api.NewUserStatsServiceHandler(statsService)
	authInterceptor := auth.NewAuthInterceptor(signer)
	path, handler := userstatsv1connect.NewUserStatsServiceHandler(
		statsHandler,
		connect.WithInterceptors(authInterceptor),
	)

	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// 4. Start Server
	addr := ":8081" // Use 8081 for Stats Service API to avoid conflict with Bid API (8080)
	logger.Info("Starting User Stats Service API", "addr", addr)

	srv := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
