package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/floroz/gavel/services/bid-service/internal/adapters/events"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Load environment variables (local overrides .env)
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		logger.Info("Shutting down worker...")
		cancel()
	}()

	// 1. Initialize Postgres Connection Pool
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

	// 2. Connect to RabbitMQ
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		logger.Error("RABBITMQ_URL is not set")
		os.Exit(1)
	}
	amqpConn, err := amqp.Dial(rabbitURL)
	if err != nil {
		logger.Error("Failed to connect to RabbitMQ", "error", err)
		os.Exit(1)
	}
	defer amqpConn.Close()
	logger.Info("RabbitMQ Connected")

	// 3. Initialize Producer
	producer, err := events.NewBidEventsProducer(pool, amqpConn, logger)
	if err != nil {
		logger.Error("Failed to create producer", "error", err)
		os.Exit(1)
	}
	defer producer.Close()

	logger.Info("Starting Bid Events Producer...")
	if runErr := producer.Run(ctx); runErr != nil {
		logger.Error("Producer failed", "error", runErr)
		// Run returns nil on context cancel.
		if ctx.Err() == nil {
			os.Exit(1)
		}
	}

	logger.Info("Worker stopped")
}
