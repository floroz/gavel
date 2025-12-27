package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"

	pkgdb "github.com/floroz/gavel/pkg/database"
	"github.com/floroz/gavel/services/user-stats-service/internal/adapters/database"
	"github.com/floroz/gavel/services/user-stats-service/internal/adapters/events"
	"github.com/floroz/gavel/services/user-stats-service/internal/domain/userstats"
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
		logger.Info("Shutting down user stats consumer...")
		cancel()
	}()

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

	// 3. Connect to RabbitMQ
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

	// 4. Start Consumer
	consumer := events.NewBidConsumer(amqpConn, statsService, logger)
	logger.Info("Starting bid consumer...")
	if err := consumer.Run(ctx); err != nil {
		logger.Error("Consumer failed", "error", err)
		// Don't exit here immediately if context was canceled?
		// Run returns nil on context cancel.
		if ctx.Err() == nil {
			os.Exit(1)
		}
	}
	logger.Info("User stats consumer stopped")
}
