package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tcandt/cloudphone-farm/backend/internal/config"
)

func main() {
	cfg := config.LoadConfig()

	if cfg.AppEnv != "development" {
		slog.Error("Devseed command rejected: APP_ENV is not 'development'", "app_env", cfg.AppEnv)
		os.Exit(1)
	}

	slog.Info("Running development database seed...", "env", cfg.AppEnv)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.PostgresURL)
	if err != nil {
		slog.Error("Failed to connect to PostgreSQL for devseed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	seedSQL, err := os.ReadFile("db/devseed/dev_seed.sql")
	if err != nil {
		slog.Error("Failed to read dev_seed.sql", "error", err)
		os.Exit(1)
	}

	if _, err := pool.Exec(ctx, string(seedSQL)); err != nil {
		slog.Error("Failed to execute dev_seed.sql", "error", err)
		os.Exit(1)
	}

	fmt.Println("Development database seed completed successfully!")
}
