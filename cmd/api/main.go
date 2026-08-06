package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cashvio/cashvio-be/internal/config"
	"github.com/cashvio/cashvio-be/internal/database"
	"github.com/cashvio/cashvio-be/internal/handler"
	"github.com/cashvio/cashvio-be/internal/repository"
	"github.com/cashvio/cashvio-be/internal/route"
	"github.com/cashvio/cashvio-be/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	if err := database.CreateDatabase(&cfg.Database); err != nil {
		log.Printf("database %s might already exist: %v", cfg.Database.DBName, err)
	} else {
		log.Printf("created database: %s", cfg.Database.DBName)
	}

	log.Printf("connecting to database: %s", cfg.Database.DBName)
	dbPool, err := database.New(&cfg.Database)
	if err != nil {
		log.Fatalf("database error: %v", err)
	}
	defer database.Close(dbPool)

	if err := runMigrations(dbPool); err != nil {
		log.Fatalf("migration error: %v", err)
	}
	log.Println("migrations completed")

	userRepo := repository.NewUserRepository(dbPool)
	userSvc := service.NewUserService(userRepo, cfg)
	userHandler := handler.NewUserHandler(userSvc, cfg)

	cardRepo := repository.NewCardRepository(dbPool)
	cardSvc := service.NewCardService(cardRepo)
	cardHandler := handler.NewCardHandler(cardSvc, cfg)

	walletRepo := repository.NewWalletRepository(dbPool)
	walletSvc := service.NewWalletService(walletRepo)
	walletHandler := handler.NewWalletHandler(walletSvc, cfg)

	transactionRepo := repository.NewTransactionRepository(dbPool)
	transactionSvc := service.NewTransactionService(transactionRepo, dbPool, walletRepo, cardRepo)
	transactionHandler := handler.NewTransactionHandler(transactionSvc, cfg)

	r := route.Setup(cfg, userHandler, cardHandler, walletHandler, transactionHandler)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Server.Port),
		Handler: r,
	}

	go func() {
		log.Printf("server starting on port %s (mode: %s)", cfg.Server.Port, cfg.Server.Mode)
		log.Printf("database: %s", cfg.Database.DBName)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown error: %v", err)
	}

	log.Println("server exited")
}

func runMigrations(pool *pgxpool.Pool) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			password VARCHAR(255) NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS cards (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			bank VARCHAR(100) NOT NULL,
			number VARCHAR(50) NOT NULL,
			masked VARCHAR(50) NOT NULL,
			balance_idr DECIMAL(16,2) NOT NULL DEFAULT 0,
			gradient VARCHAR(100),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS wallets (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(100) NOT NULL,
			number VARCHAR(50),
			masked VARCHAR(50),
			balance_idr DECIMAL(16,2) NOT NULL DEFAULT 0,
			tone VARCHAR(200),
			status VARCHAR(20) NOT NULL DEFAULT 'active',
			"primary" BOOLEAN NOT NULL DEFAULT false,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			name VARCHAR(100) NOT NULL,
			amount DECIMAL(16,2) NOT NULL,
			type VARCHAR(10) NOT NULL CHECK (type IN ('income','expense')),
			category VARCHAR(30) NOT NULL CHECK (category IN ('income','salary','shopping','groceries','subscription','travel','transfer')),
			status VARCHAR(20) NOT NULL DEFAULT 'completed' CHECK (status IN ('completed','pending','failed')),
			account_type VARCHAR(10) NOT NULL CHECK (account_type IN ('wallet','card')),
			account_id UUID NOT NULL,
			date DATE NOT NULL DEFAULT CURRENT_DATE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, m := range migrations {
		if _, err := pool.Exec(ctx, m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	return nil
}
