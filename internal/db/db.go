// Package db provides PostgreSQL database connectivity and operations for Cortex.
package db

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/johnswift/cortex/migrations"
)

// DB wraps a pgxpool connection pool with Cortex-specific operations.
type DB struct {
	pool        *pgxpool.Pool
	tenantID    string
	workspaceID string
}

// New creates a new DB instance with the given connection URL and tenant ID.
// Uses "default" workspace for backward compatibility.
func New(ctx context.Context, databaseURL, tenantID string) (*DB, error) {
	return NewWithWorkspace(ctx, databaseURL, tenantID, "default")
}

// NewWithWorkspace creates a new DB instance with the given connection URL, tenant ID, and workspace ID.
func NewWithWorkspace(ctx context.Context, databaseURL, tenantID, workspaceID string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	db := &DB{
		pool:        pool,
		tenantID:    tenantID,
		workspaceID: workspaceID,
	}

	return db, nil
}

// Close closes the database connection pool.
func (db *DB) Close() {
	db.pool.Close()
}

// Pool returns the underlying connection pool for advanced operations.
func (db *DB) Pool() *pgxpool.Pool {
	return db.pool
}

// TenantID returns the configured tenant ID.
func (db *DB) TenantID() string {
	return db.tenantID
}

// WorkspaceID returns the configured workspace ID.
func (db *DB) WorkspaceID() string {
	return db.workspaceID
}

// Migrate runs all embedded SQL migrations in order.
func (db *DB) Migrate(ctx context.Context) error {
	// Get all SQL files from embedded FS
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}

	// Filter and sort SQL files
	var sqlFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			sqlFiles = append(sqlFiles, entry.Name())
		}
	}
	sort.Strings(sqlFiles)

	// Execute each migration
	for _, filename := range sqlFiles {
		content, err := fs.ReadFile(migrations.FS, filename)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}

		if _, err := db.pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("execute migration %s: %w", filename, err)
		}
	}

	return nil
}

// WithTx executes a function within a transaction.
func (db *DB) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
