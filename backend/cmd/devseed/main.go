package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tcandt/cloudphone-farm/backend/internal/config"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
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

	// 1. Seed Dev Organization
	orgSQL := `
		INSERT INTO organizations (organization_id, name, slug) VALUES
		('org_pcp_enterprise_01', 'Enterprise Cloud Farm', 'enterprise-cloud-farm')
		ON CONFLICT (organization_id) DO NOTHING;
	`
	if _, err := pool.Exec(ctx, orgSQL); err != nil {
		slog.Error("Failed to seed dev organization", "error", err)
		os.Exit(1)
	}

	// 2. Generate real Argon2id hash for dev owner password
	devPassword := os.Getenv("DEV_SEED_OWNER_PASSWORD")
	if devPassword == "" {
		devPassword = "pcp_secure_pass_2026"
	}

	realHash, err := crypto.HashPassword(devPassword)
	if err != nil {
		slog.Error("Failed to hash dev owner password", "error", err)
		os.Exit(1)
	}

	userSQL := `
		INSERT INTO users (user_id, email, password_hash, display_name) VALUES
		('usr_owner_01', 'owner@phonecontrol.io', $1, 'Minh Tuấn (Owner)')
		ON CONFLICT (user_id) DO UPDATE SET password_hash = EXCLUDED.password_hash;
	`
	if _, err := pool.Exec(ctx, userSQL, realHash); err != nil {
		slog.Error("Failed to seed dev user", "error", err)
		os.Exit(1)
	}

	// 3. Seed Membership & Role assignment
	membershipSQL := `
		INSERT INTO organization_memberships (membership_id, organization_id, user_id, status) VALUES
		('mem_owner_01', 'org_pcp_enterprise_01', 'usr_owner_01', 'active')
		ON CONFLICT (membership_id) DO NOTHING;

		INSERT INTO user_roles (membership_id, role_id) VALUES
		('mem_owner_01', 'role_sys_owner')
		ON CONFLICT DO NOTHING;
	`
	if _, err := pool.Exec(ctx, membershipSQL); err != nil {
		slog.Error("Failed to seed dev membership", "error", err)
		os.Exit(1)
	}

	slog.Info("Development database seed completed successfully!", "user", "owner@phonecontrol.io")
}
