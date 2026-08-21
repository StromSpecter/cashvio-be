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
	"github.com/cashvio/cashvio-be/internal/middleware"
	"github.com/cashvio/cashvio-be/internal/model"
	"github.com/cashvio/cashvio-be/internal/payment"
	"github.com/cashvio/cashvio-be/internal/repository"
	"github.com/cashvio/cashvio-be/internal/route"
	"github.com/cashvio/cashvio-be/internal/service"
	"github.com/google/uuid"
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

	cashRepo := repository.NewCashRepository(dbPool)
	cashSvc := service.NewCashService(cashRepo, dbPool, walletRepo, cardRepo)
	cashHandler := handler.NewCashHandler(cashSvc, cfg)

	transactionRepo := repository.NewTransactionRepository(dbPool)
	transactionSvc := service.NewTransactionService(transactionRepo, dbPool, walletRepo, cardRepo, cashRepo)
	transactionHandler := handler.NewTransactionHandler(transactionSvc, cfg)

	transferRepo := repository.NewTransferRepository(dbPool)
	transferSvc := service.NewTransferService(transferRepo, dbPool, walletRepo, cardRepo)
	transferHandler := handler.NewTransferHandler(transferSvc, cfg)

	dashboardSvc := service.NewDashboardService(transactionRepo, walletRepo, cardRepo, cashRepo)
	dashboardHandler := handler.NewDashboardHandler(dashboardSvc, cfg)

	stockPriceRepo := repository.NewStockPriceRepository(dbPool)

	investmentRepo := repository.NewInvestmentRepository(dbPool)
	investmentSvc := service.NewInvestmentService(investmentRepo, transactionRepo, dbPool, cfg, stockPriceRepo, walletRepo, cardRepo, cashRepo)
	investmentHandler := handler.NewInvestmentHandler(investmentSvc, cfg)

	premiumRepo := repository.NewPremiumRepository(dbPool)
	premiumProvider := payment.NewProvider(map[string]string{
		"provider":       cfg.Payment.Provider,
		"webhook_secret": cfg.Payment.WebhookSecret,
	})
	premiumSvc := service.NewPremiumService(premiumRepo, userRepo, premiumProvider)
	premiumHandler := handler.NewPremiumHandler(premiumSvc, premiumProvider, cfg)

	premiumGuard := middleware.RequirePremium(func(ctx context.Context, id uuid.UUID) (*model.User, error) {
		return userRepo.GetByID(ctx, id)
	})

	r := route.Setup(cfg, userHandler, cardHandler, walletHandler, transactionHandler, transferHandler, cashHandler, dashboardHandler, investmentHandler, premiumHandler, premiumGuard)

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
		`CREATE TABLE IF NOT EXISTS transfers (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			from_type VARCHAR(10) NOT NULL CHECK (from_type IN ('wallet','card')),
			from_id UUID NOT NULL,
			to_type VARCHAR(10) NOT NULL CHECK (to_type IN ('wallet','card')),
			to_id UUID NOT NULL,
			amount DECIMAL(16,2) NOT NULL CHECK (amount > 0),
			fee DECIMAL(16,2) NOT NULL DEFAULT 0 CHECK (fee >= 0),
			note VARCHAR(255),
			date DATE NOT NULL DEFAULT CURRENT_DATE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`ALTER TABLE transfers ADD COLUMN IF NOT EXISTS fee DECIMAL(16,2) NOT NULL DEFAULT 0 CHECK (fee >= 0)`,
		`CREATE TABLE IF NOT EXISTS cashes (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
			balance_idr DECIMAL(16,2) NOT NULL DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS cash_withdrawals (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			from_type VARCHAR(10) NOT NULL CHECK (from_type IN ('wallet','card')),
			from_id UUID NOT NULL,
			amount DECIMAL(16,2) NOT NULL CHECK (amount > 0),
			fee DECIMAL(16,2) NOT NULL DEFAULT 0 CHECK (fee >= 0),
			note VARCHAR(255),
			date DATE NOT NULL DEFAULT CURRENT_DATE,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_account_type_check`,
		`ALTER TABLE transactions ADD CONSTRAINT transactions_account_type_check CHECK (account_type IN ('wallet','card','cash'))`,
		`ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_category_check`,
		`ALTER TABLE transactions ADD CONSTRAINT transactions_category_check CHECK (category IN ('income','salary','shopping','groceries','subscription','travel','transfer','freelance','gift','bonus','food','transportation','housing','entertainment','health','education','pets','investment'))`,
		`CREATE TABLE IF NOT EXISTS investments (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			type VARCHAR(30) NOT NULL DEFAULT 'stock' CHECK (type IN ('stock','gold')),
			name VARCHAR(100) NOT NULL,
			ticker VARCHAR(20) NOT NULL DEFAULT '',
			app VARCHAR(50) NOT NULL DEFAULT '',
			account_type VARCHAR(10) NOT NULL CHECK (account_type IN ('wallet','card','cash')),
			account_id UUID NOT NULL,
			units DECIMAL(16,4) NOT NULL DEFAULT 0,
			buy_price DECIMAL(16,2) NOT NULL DEFAULT 0,
			date DATE NOT NULL DEFAULT CURRENT_DATE,
			transaction_id UUID NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`ALTER TABLE investments DROP COLUMN IF EXISTS current_price`,
		`ALTER TABLE investments ALTER COLUMN account_type DROP NOT NULL`,
		`ALTER TABLE investments ALTER COLUMN account_id DROP NOT NULL`,
		`ALTER TABLE investments ALTER COLUMN transaction_id DROP NOT NULL`,
		`DELETE FROM investments WHERE type NOT IN ('stock','gold')`,
		`ALTER TABLE investments DROP CONSTRAINT IF EXISTS investments_type_check`,
		`ALTER TABLE investments ADD CONSTRAINT investments_type_check CHECK (type IN ('stock','gold'))`,
		`CREATE TABLE IF NOT EXISTS stock_prices (
			id UUID PRIMARY KEY,
			symbol VARCHAR(20) NOT NULL,
			date DATE NOT NULL,
			open DECIMAL(16,2) NOT NULL DEFAULT 0,
			high DECIMAL(16,2) NOT NULL DEFAULT 0,
			low DECIMAL(16,2) NOT NULL DEFAULT 0,
			close DECIMAL(16,2) NOT NULL DEFAULT 0,
			volume BIGINT NOT NULL DEFAULT 0,
			change DECIMAL(16,2) NOT NULL DEFAULT 0,
			change_pct DECIMAL(16,4) NOT NULL DEFAULT 0,
			fetched_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (symbol, date)
		)`,
		`DROP TABLE IF EXISTS budgets CASCADE`,
		`DROP TABLE IF EXISTS category_budgets CASCADE`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS role VARCHAR(20) NOT NULL DEFAULT 'free' CHECK (role IN ('free','premium'))`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS premium_expires_at TIMESTAMP WITH TIME ZONE`,
		`CREATE TABLE IF NOT EXISTS premium_orders (
			id UUID PRIMARY KEY,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			plan VARCHAR(20) NOT NULL,
			amount DECIMAL(16,2) NOT NULL,
			currency VARCHAR(3) NOT NULL DEFAULT 'IDR',
			duration_days INT NOT NULL,
			external_id VARCHAR(100) UNIQUE NOT NULL,
			qris_string TEXT NOT NULL DEFAULT '',
			qris_image_url TEXT NOT NULL DEFAULT '',
			status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','paid','expired','failed')),
			paid_at TIMESTAMP WITH TIME ZONE,
			premium_expires_at TIMESTAMP WITH TIME ZONE,
			expires_at TIMESTAMP WITH TIME ZONE,
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
