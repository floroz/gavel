package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/floroz/auction-system/internal/auction"
	"github.com/floroz/auction-system/internal/infra/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()

	// 1. Initialize Postgres Connection Pool
	dbConfig, err := pgxpool.ParseConfig("postgres://user:password@localhost:5432/auction_db")
	if err != nil {
		log.Fatalf("Unable to parse database config: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, dbConfig)
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Unable to ping database: %v", err)
	}
	log.Println("✅ Postgres Connected")

	// 2. Check RabbitMQ
	mq, err := amqp091.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatalf("RabbitMQ failed: %v", err)
	}
	defer mq.Close()
	log.Println("✅ RabbitMQ Connected")

	// 3. Check Redis
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Redis failed: %v", err)
	}
	log.Println("✅ Redis Connected")

	// 4. Initialize Repositories (Infrastructure Layer)
	// Set lock timeout to 3 seconds to prevent indefinite waiting
	txManager := database.NewPostgresTransactionManager(pool, 3*time.Second)
	bidRepo := database.NewPostgresBidRepository(pool)
	itemRepo := database.NewPostgresItemRepository(pool)
	outboxRepo := database.NewPostgresOutboxRepository(pool)

	// 5. Initialize Service (Domain Layer)
	auctionService := auction.NewAuctionService(txManager, bidRepo, itemRepo, outboxRepo)

	log.Println("✅ Services Initialized")

	// 6. Demo: Create a test item and place a bid
	if err := demoPlaceBid(ctx, pool, auctionService); err != nil {
		log.Printf("Demo failed: %v", err)
	}

	log.Println("Milestone 2: Transactional Outbox Pattern implemented.")
	log.Println("Next: Implement the Outbox Relay worker to publish events to RabbitMQ.")
}

// demoPlaceBid demonstrates the PlaceBid functionality
func demoPlaceBid(ctx context.Context, pool *pgxpool.Pool, service *auction.AuctionService) error {
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

	log.Printf("✅ Created test item: %s", itemID)

	// Place a bid
	userID := uuid.New()
	cmd := auction.PlaceBidCommand{
		ItemID: itemID,
		UserID: userID,
		Amount: int64(15000), // $150.00
	}

	bid, err := service.PlaceBid(ctx, cmd)
	if err != nil {
		return fmt.Errorf("failed to place bid: %w", err)
	}

	log.Printf("✅ Bid placed successfully: ID=%s, Amount=%d", bid.ID, bid.Amount)

	// Verify the outbox event was created
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM outbox_events WHERE event_type = $1", auction.EventTypeBidPlaced).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to count outbox events: %w", err)
	}

	log.Printf("✅ Outbox events created: %d", count)

	return nil
}
