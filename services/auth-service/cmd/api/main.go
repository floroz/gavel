package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/floroz/gavel/pkg/auth"
	pkgdb "github.com/floroz/gavel/pkg/database"
	pkgevents "github.com/floroz/gavel/pkg/events"
	"github.com/floroz/gavel/pkg/proto/auth/v1/authv1connect"
	"github.com/floroz/gavel/services/auth-service/internal/adapters/api"
	"github.com/floroz/gavel/services/auth-service/internal/adapters/database"

	"github.com/floroz/gavel/services/auth-service/internal/domain/users"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Load environment variables (local overrides .env)
	_ = godotenv.Load(".env.local")
	_ = godotenv.Load()

	ctx := context.Background()

	// 1. Load Keys
	privateKeyPath := os.Getenv("AUTH_PRIVATE_KEY_PATH")
	publicKeyPath := os.Getenv("AUTH_PUBLIC_KEY_PATH")

	if privateKeyPath == "" || publicKeyPath == "" {
		logger.Error("AUTH_PRIVATE_KEY_PATH and AUTH_PUBLIC_KEY_PATH must be set")
		os.Exit(1)
	}

	privateKeyPEM, err := os.ReadFile(privateKeyPath)
	if err != nil {
		logger.Error("Failed to read private key", "path", privateKeyPath, "error", err)
		os.Exit(1)
	}

	publicKeyPEM, err := os.ReadFile(publicKeyPath)
	if err != nil {
		logger.Error("Failed to read public key", "path", publicKeyPath, "error", err)
		os.Exit(1)
	}

	signer, err := auth.NewSigner(privateKeyPEM, publicKeyPEM)
	if err != nil {
		logger.Error("Failed to create signer", "error", err)
		os.Exit(1)
	}

	// 2. Initialize Postgres Connection Pool
	dbURL := os.Getenv("AUTH_DB_URL")
	if dbURL == "" {
		logger.Error("AUTH_DB_URL is not set")
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

	// 3. Initialize RabbitMQ
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

	rabbitPublisher, err := pkgevents.NewRabbitMQPublisher(amqpConn)
	if err != nil {
		logger.Error("Failed to create RabbitMQ publisher", "error", err)
		os.Exit(1)
	}
	defer rabbitPublisher.Close()

	// 4. Initialize Repositories
	txManager := pkgdb.NewPostgresTransactionManager(pool, 5*time.Second)
	userRepo := database.NewPostgresUserRepository(pool)
	tokenRepo := database.NewPostgresTokenRepository(pool)
	outboxRepo := database.NewPostgresOutboxRepository(pool)

	// 5. Initialize Service
	authService := users.NewService(userRepo, tokenRepo, outboxRepo, signer, txManager)

	// 6. Start Outbox Relay
	outboxRelay := pkgevents.NewOutboxRelay(
		outboxRepo,
		rabbitPublisher,
		txManager,
		10,               // batch size
		5*time.Second,    // interval
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

	// 7. Initialize API Handler (ConnectRPC)
	authHandler := api.NewAuthServiceHandler(authService)
	path, connectHandler := authv1connect.NewAuthServiceHandler(authHandler)

	mux := http.NewServeMux()
	mux.Handle(path, connectHandler)

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Expose Public Key (JWKS)
	// For simplicity, we just serve the PEM file content for now on a specific endpoint
	// In a real system, we'd serve a standard JWKS JSON.
	mux.HandleFunc("/.well-known/public-key", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write(publicKeyPEM)
	})

	// 6. Start Server
	addr := ":8080"
	logger.Info("Starting Auth Service API", "addr", addr)

	srv := &http.Server{
		Addr:    addr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("Server failed", "error", err)
		os.Exit(1)
	}
}
