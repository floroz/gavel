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
	"github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/floroz/gavel/pkg/auth"
	pkgdb "github.com/floroz/gavel/pkg/database"
	pkgevents "github.com/floroz/gavel/pkg/events"
	"github.com/floroz/gavel/pkg/proto/bids/v1/bidsv1connect"
	"github.com/floroz/gavel/services/bid-service/internal/adapters/api"
	"github.com/floroz/gavel/services/bid-service/internal/adapters/database"
	"github.com/floroz/gavel/services/bid-service/internal/domain/bids"
	"github.com/floroz/gavel/services/bid-service/internal/domain/items"
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
	dbURL := os.Getenv("BID_DB_URL")

	if dbURL == "" {
		logger.Error("BID_DB_URL is not set")
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

	if pingErr := pool.Ping(ctx); pingErr != nil {
		logger.Error("Unable to ping database", "error", pingErr)
		os.Exit(1)
	}
	logger.Info("Postgres Connected")

	// 2. Check RabbitMQ (Optional for API, but good for health)
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		logger.Error("RABBITMQ_URL is not set")
		os.Exit(1)
	}

	amqpConn, err := amqp091.Dial(rabbitURL)
	if err != nil {
		logger.Error("Failed to connect to RabbitMQ", "error", err)
		os.Exit(1)
	}
	defer amqpConn.Close()
	logger.Info("RabbitMQ Connected")

	rabbitPublisher, err := pkgevents.NewRabbitMQPublisher(amqpConn)
	if err != nil {
		logger.Error("Failed to create RabbitMQ publisher", "error", err)
		os.Exit(1)
	}
	defer rabbitPublisher.Close()

	// 3. Check Redis (Optional for API, but good for health)
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" {
		rdb := redis.NewClient(&redis.Options{Addr: redisURL})
		if err := rdb.Ping(ctx).Err(); err != nil {
			logger.Warn("Redis connection failed (API might still work)", "error", err)
		} else {
			logger.Info("Redis Connected")
		}
	}

	// 4. Initialize Repositories (Infrastructure Layer)
	txManager := pkgdb.NewPostgresTransactionManager(pool, 3*time.Second)
	bidRepo := database.NewPostgresBidRepository(pool)
	itemRepo := database.NewPostgresItemRepository(pool)
	outboxRepo := database.NewPostgresOutboxRepository(pool)

	// 5. Initialize Service (Domain Layer)
	auctionService := bids.NewAuctionService(txManager, bidRepo, itemRepo, outboxRepo)
	itemService := items.NewService(itemRepo)

	// 7. Initialize API Handler (ConnectRPC) with auth interceptor
	bidHandler := api.NewBidServiceHandler(auctionService, itemService, bidRepo)

	// Configure public routes (no auth required)
	publicRoutes := map[string]bool{
		"/bids.v1.BidService/GetItem":     true,
		"/bids.v1.BidService/ListItems":   true,
		"/bids.v1.BidService/GetItemBids": true,
	}

	authInterceptor := auth.NewAuthInterceptorWithPublicRoutes(signer, publicRoutes)
	path, handler := bidsv1connect.NewBidServiceHandler(
		bidHandler,
		connect.WithInterceptors(authInterceptor),
	)

	// 7. Start Outbox Relay
	outboxRelay := pkgevents.NewOutboxRelay(
		outboxRepo,
		rabbitPublisher,
		txManager,
		10,               // batch size
		1*time.Second,    // interval
		"auction.events", // exchange
		logger,
	)

	// Run relay in background
	go func() {
		logger.Info("Starting Outbox Relay...")
		if err := outboxRelay.Run(ctx); err != nil {
			logger.Error("Outbox Relay stopped", "error", err)
		}
	}()

	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// 7. Start Server
	addr := ":8080"
	logger.Info("Starting Bid Service API", "addr", addr)

	// Use h2c for HTTP/2 without TLS (common for internal services / local dev)
	srv := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
