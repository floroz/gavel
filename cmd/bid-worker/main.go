package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/floroz/auction-system/internal/infra/database"
	"github.com/floroz/auction-system/internal/infra/events"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

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
	dbConfig, err := pgxpool.ParseConfig("postgres://user:password@localhost:5432/auction_db")
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

	if err := pool.Ping(ctx); err != nil {
		logger.Error("Unable to ping database", "error", err)
		os.Exit(1)
	}
	logger.Info("Postgres Connected")

	// 2. Initialize RabbitMQ Publisher
	rabbitPublisher, err := events.NewRabbitMQPublisher("amqp://guest:guest@localhost:5672/")
	if err != nil {
		logger.Error("Failed to initialize RabbitMQ publisher", "error", err)
		os.Exit(1)
	}
	defer rabbitPublisher.Close()
	logger.Info("RabbitMQ Connected")

	// 3. Initialize Repositories and Relay
	// Set lock timeout to 3 seconds
	txManager := database.NewPostgresTransactionManager(pool, 3*time.Second)
	outboxRepo := database.NewPostgresOutboxRepository(pool)

	relay := events.NewOutboxRelay(
		outboxRepo,
		rabbitPublisher,
		txManager,
		10,                   // Batch size
		500*time.Millisecond, // Polling interval
		logger,
	)

	logger.Info("Starting Outbox Relay Worker...")
	if err := relay.Run(ctx); err != nil {
		logger.Error("Relay failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Worker stopped")
}
