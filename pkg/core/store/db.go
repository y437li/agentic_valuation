package store

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	pool *pgxpool.Pool
	once sync.Once
)

// InitDB initializes the database connection pool using the DATABASE_URL environment variable
func InitDB(ctx context.Context) error {
	var err error
	once.Do(func() {
		dbURL := os.Getenv("DATABASE_URL")
		if dbURL == "" {
			err = fmt.Errorf("DATABASE_URL environment variable not set")
			return
		}

		config, parseErr := pgxpool.ParseConfig(dbURL)
		if parseErr != nil {
			err = fmt.Errorf("failed to parse database config: %w", parseErr)
			return
		}

		pool, err = pgxpool.NewWithConfig(ctx, config)
	})
	return err
}

// GetPool returns the database connection pool
func GetPool() *pgxpool.Pool {
	return pool
}

// Close closes the database connection pool
func Close() {
	if pool != nil {
		pool.Close()
	}
}
