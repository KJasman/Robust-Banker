package database

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TimescaleDBHandler struct {
	pool *pgxpool.Pool
}

func NewTimescaleDBHandler() (*TimescaleDBHandler, error) {
	host := os.Getenv("TIMESCALE_HOST")
	if host == "" {
		host = "localhost"
	}

	port := os.Getenv("TIMESCALE_PORT")
	if port == "" {
		port = "5432"
	}

	user := os.Getenv("TIMESCALE_USER")
	if user == "" {
		user = "postgres"
	}

	password := os.Getenv("TIMESCALE_PASSWORD")
	if password == "" {
		password = "postgres"
	}

	dbname := os.Getenv("TIMESCALE_DB")
	if dbname == "" {
		dbname = "order_history"
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, password, host, port, dbname)

	// Create a connection pool
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("unable to parse connection string: %v", err)
	}

	// Set pool configuration
	config.MaxConns = 10
	config.MinConns = 2
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	// Create context with timeout for connection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %v", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("unable to ping database: %v", err)
	}

	return &TimescaleDBHandler{pool: pool}, nil
}

func (h *TimescaleDBHandler) Close() {
	if h.pool != nil {
		h.pool.Close()
	}
}

func (h *TimescaleDBHandler) RunMigrations() error {
	// Read the migration file
	migrationSQL, err := os.ReadFile("migrations/001_create_tables.sql")
	if err != nil {
		return fmt.Errorf("failed to read migration file: %v", err)
	}

	// Execute the migration script
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, err = h.pool.Exec(ctx, string(migrationSQL))
	if err != nil {
		return fmt.Errorf("failed to execute migrations: %v", err)
	}

	log.Println("Successfully applied migrations")
	return nil
}

func (h *TimescaleDBHandler) GetDB() *pgxpool.Pool {
	return h.pool
}
