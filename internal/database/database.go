package database

import (
	"context"
	"fmt"
	"time"

	"github.com/cashvio/cashvio-be/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func New(cfg *config.DatabaseConfig) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(context.Background(), cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database %s: %w", cfg.DBName, err)
	}

	return pool, nil
}

func CreateDatabase(cfg *config.DatabaseConfig) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.RootDSN())
	if err != nil {
		return fmt.Errorf("failed to connect to postgres db: %w", err)
	}
	defer pool.Close()

	query := fmt.Sprintf("CREATE DATABASE %s", cfg.DBName)
	if _, err := pool.Exec(ctx, query); err != nil {
		return fmt.Errorf("failed to create database %s: %w", cfg.DBName, err)
	}

	return nil
}

func Close(pool *pgxpool.Pool) {
	pool.Close()
}
