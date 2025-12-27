package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresTransactionManager implements database.TransactionManager using pgx
type PostgresTransactionManager struct {
	pool        *pgxpool.Pool
	lockTimeout time.Duration
}

// NewPostgresTransactionManager creates a new PostgreSQL transaction manager
// lockTimeout: maximum time to wait for a lock (0 = no timeout)
func NewPostgresTransactionManager(pool *pgxpool.Pool, lockTimeout time.Duration) *PostgresTransactionManager {
	return &PostgresTransactionManager{
		pool:        pool,
		lockTimeout: lockTimeout,
	}
}

// BeginTx starts a new transaction with configured lock timeout
func (m *PostgresTransactionManager) BeginTx(ctx context.Context) (pgx.Tx, error) {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}

	// Set lock timeout for this transaction
	if m.lockTimeout > 0 {
		timeoutMs := int(m.lockTimeout.Milliseconds())
		_, err = tx.Exec(ctx, fmt.Sprintf("SET LOCAL lock_timeout = '%dms'", timeoutMs))
		if err != nil {
			_ = tx.Rollback(ctx)
			return nil, fmt.Errorf("failed to set lock timeout: %w", err)
		}
	}

	return tx, nil
}
