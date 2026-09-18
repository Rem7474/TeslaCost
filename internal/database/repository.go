package database

import (
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound = errors.New("record not found")
)

// Repository encapsulates database operations.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository instance.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}
