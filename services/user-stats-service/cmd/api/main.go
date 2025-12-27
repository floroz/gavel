package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

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

	// 1. Initialize Postgres Connection Pool
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

	// 3. Initialize API Handler
	statsHandler := api.NewUserStatsServiceHandler(statsService)
	path, handler := userstatsv1connect.NewUserStatsServiceHandler(statsHandler)

	mux := http.NewServeMux()
	mux.Handle(path, handler)

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
