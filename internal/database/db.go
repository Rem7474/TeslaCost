package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/teslacost/teslacost/migrations"
)

// DB encapsulates the pgx connection pool.
type DB struct {
	Pool *pgxpool.Pool
}

// Connect initializes a connection pool to PostgreSQL.
func Connect(ctx context.Context, databaseURL string) (*DB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database config: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 2
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database pool: %w", err)
	}

	// Ping database
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		return nil, fmt.Errorf("database ping failed: %w", err)
	}

	log.Println("[database] Connected successfully to PostgreSQL")
	return &DB{Pool: pool}, nil
}

// Migrate executes embedded SQL migration scripts to ensure the database schema is up-to-date.
func (db *DB) Migrate(ctx context.Context) error {
	schemaSQL, err := migrations.FS.ReadFile("000001_init_schema.up.sql")
	if err != nil {
		return fmt.Errorf("failed to read embedded schema migration: %w", err)
	}

	_, err = db.Pool.Exec(ctx, string(schemaSQL))
	if err != nil {
		return fmt.Errorf("failed to execute schema migration: %w", err)
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
