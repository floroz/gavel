package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"

	"github.com/floroz/auction-system/internal/bids"
	"github.com/floroz/auction-system/internal/infra/database"
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
	dbURL := os.Getenv("DATABASE_URL")

	if dbURL == "" {
		logger.Error("DATABASE_URL is not set")
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

	// 2. Check RabbitMQ
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		logger.Error("RABBITMQ_URL is not set")
		os.Exit(1)
	}

	mq, err := amqp091.Dial(rabbitURL)
	if err != nil {
		logger.Error("RabbitMQ failed", "error", err)
		os.Exit(1)
	}
	defer mq.Close()
	logger.Info("RabbitMQ Connected")

	// 3. Check Redis
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		logger.Error("REDIS_URL is not set")
		os.Exit(1)
	}
	rdb := redis.NewClient(&redis.Options{Addr: redisURL})
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("Redis failed", "error", err)
		os.Exit(1)
	}
	logger.Info("Redis Connected")

	// 4. Initialize Repositories (Infrastructure Layer)
	// Set lock timeout to 3 seconds to prevent indefinite waiting
	txManager := database.NewPostgresTransactionManager(pool, 3*time.Second)
	bidRepo := database.NewPostgresBidRepository(pool)
	itemRepo := database.NewPostgresItemRepository(pool)
	outboxRepo := database.NewPostgresOutboxRepository(pool)

	// 5. Initialize Service (Domain Layer)
	auctionService := bids.NewAuctionService(txManager, bidRepo, itemRepo, outboxRepo)

	logger.Info("Services Initialized")

	// 6. Demo: Create a test item and place a bid
	if err := demoPlaceBid(ctx, pool, auctionService, logger); err != nil {
		logger.Error("Demo failed", "error", err)
	}

	logger.Info("Milestone 2: Transactional Outbox Pattern implemented.")
	logger.Info("Next: Implement the Outbox Relay worker to publish events to RabbitMQ.")
}

// demoPlaceBid demonstrates the PlaceBid functionality
func demoPlaceBid(ctx context.Context, pool *pgxpool.Pool, service *bids.AuctionService, logger *slog.Logger) error {
	// Create a test item
	itemID := uuid.New()
	query := `
		INSERT INTO items (id, title, description, start_price, current_highest_bid, end_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
	`
	_, err := pool.Exec(ctx, query,
		itemID,
		"Vintage Watch",
		"A beautiful vintage watch from the 1960s",
		int64(10000), // $100.00
		int64(0),
		time.Now().Add(24*time.Hour),
	)
	if err != nil {
		return fmt.Errorf("failed to create test item: %w", err)
	}

	logger.Info("Created test item", "item_id", itemID)

	// Place a bid
	userID := uuid.New()
	cmd := bids.PlaceBidCommand{
		ItemID: itemID,
		UserID: userID,
		Amount: int64(15000), // $150.00
	}

	bid, err := service.PlaceBid(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to place bid: %w", err)
	}

	logger.Info("Bid placed successfully", "bid_id", bid.ID, "amount", bid.Amount)

	// Verify the outbox event was created
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox_events WHERE event_type = $1", bids.EventTypeBidPlaced).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count outbox events: %w", err)
	}

	logger.Info("Outbox events created", "count", count)

	return nil
}
