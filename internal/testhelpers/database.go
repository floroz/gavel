package testhelpers

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestDatabase represents a test database with connection pool and cleanup function
type TestDatabase struct {
	Pool    *pgxpool.Pool
	cleanup func()
}

// Close cleans up the test database and terminates the container
func (db *TestDatabase) Close() {
	if db.cleanup != nil {
		db.cleanup()
	}
}

// NewTestDatabase creates a new PostgreSQL test database with testcontainers
// It starts a container, runs migrations, and returns a ready-to-use database
func NewTestDatabase(t *testing.T, migrationsDir string) *TestDatabase {
	t.Helper()

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err, "Failed to start postgres container")

	// Get connection string
	connStr, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "Failed to get connection string")

	// Create pgx connection pool
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err, "Failed to create connection pool")

	// Run migrations using goose
	runMigrations(t, pool, migrationsDir)

	// Cleanup function
	cleanup := func() {
		pool.Close()
		if err := postgresContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate container: %v", err)
		}
	}

	return &TestDatabase{
		Pool:    pool,
		cleanup: cleanup,
	}
}

// runMigrations applies database migrations using goose
func runMigrations(t *testing.T, pool *pgxpool.Pool, migrationsDir string) {
	t.Helper()

	// Goose requires a *sql.DB, so we create one from the pgx pool
	connConfig := pool.Config().ConnConfig
	connStr := stdlib.RegisterConnConfig(connConfig)
	db, err := sql.Open("pgx", connStr)
	require.NoError(t, err, "Failed to create sql.DB for goose")
	defer db.Close()

	// Set goose dialect
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("Failed to set goose dialect: %v", err)
	}

	// Run migrations
	if err := goose.Up(db, migrationsDir); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	t.Logf("Migrations applied successfully from: %s", migrationsDir)
}

// CleanDatabase truncates all tables to reset state between tests
// Useful when reusing a database across multiple tests
func CleanDatabase(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	ctx := context.Background()
	queries := []string{
		"TRUNCATE TABLE bids CASCADE",
		"TRUNCATE TABLE items CASCADE",
		"TRUNCATE TABLE outbox_events CASCADE",
	}

	for _, query := range queries {
		_, err := pool.Exec(ctx, query)
		require.NoError(t, err, "Failed to truncate table: %s", query)
	}
}

