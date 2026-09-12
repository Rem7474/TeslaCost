package database

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/teslacost/teslacost/migrations"
)

// DB encapsulates the pgx connection pool.
type DB struct {
	Pool *pgxpool.Pool
}

// Connect initializes a connection pool to PostgreSQL with automatic retries on startup.
func Connect(ctx context.Context, databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 2
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	var pool *pgxpool.Pool
	var lastErr error

	// Retry loop (up to 30s) to allow PostgreSQL container initialization on first boot
	maxAttempts := 15
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while connecting to database: %w", ctx.Err())
		default:
		}

		pool, err = pgxpool.NewWithConfig(ctx, config)
		if err == nil {
			pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			err = pool.Ping(pingCtx)
			cancel()
			if err == nil {
				log.Println("[database] Connected successfully to PostgreSQL")
				return &DB{Pool: pool}, nil
			}
			pool.Close()
		}

		lastErr = err
		if attempt < maxAttempts {
			log.Printf("[database] Waiting for PostgreSQL to be ready (attempt %d/%d): %v", attempt, maxAttempts, err)
			time.Sleep(2 * time.Second)
		}
	}

	return nil, fmt.Errorf("database connection failed after %d attempts: %w", maxAttempts, lastErr)
}

// Migrate executes embedded SQL migration scripts to ensure the database schema is up-to-date.
func (db *DB) Migrate(ctx context.Context) error {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("failed to read embedded migrations directory: %w", err)
	}

	var upFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			upFiles = append(upFiles, entry.Name())
		}
	}
	sort.Strings(upFiles)

	for _, file := range upFiles {
		schemaSQL, err := migrations.FS.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read embedded schema migration %s: %w", file, err)
		}

		_, err = db.Pool.Exec(ctx, string(schemaSQL))
		if err != nil {
			return fmt.Errorf("failed to execute schema migration %s: %w", file, err)
		}
		log.Printf("[database] Applied migration: %s", file)
	}

	log.Println("[database] Schema migrations executed successfully")
	return nil
}

// Close closes the connection pool.
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
		log.Println("[database] Connection pool closed")
	}
}
