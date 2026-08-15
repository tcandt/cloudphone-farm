package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("Starting Database Migration Tool for PCP Backend")

	dbURL := os.Getenv("POSTGRES_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		dbURL = "postgres://pcp:pcp_password@localhost:5432/phone_farm?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("Failed to connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		slog.Error("Failed to ping PostgreSQL", "error", err)
		os.Exit(1)
	}

	migrationsDir := filepath.Join("db", "migrations")
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		migrationsDir = filepath.Join(".", "backend", "db", "migrations")
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		slog.Error("Failed to read migrations directory", "dir", migrationsDir, "error", err)
		os.Exit(1)
	}

	var upFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			upFiles = append(upFiles, entry.Name())
		}
	}
	sort.Strings(upFiles)

	slog.Info("Found migration files to apply", "count", len(upFiles), "files", upFiles)

	// Ensure pcp_schema_migrations table exists upfront
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS pcp_schema_migrations (
			version BIGINT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			checksum VARCHAR(64) NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		slog.Error("Failed to initialize pcp_schema_migrations table", "error", err)
		os.Exit(1)
	}

	for _, file := range upFiles {
		filePath := filepath.Join(migrationsDir, file)
		sqlBytes, err := os.ReadFile(filePath)
		if err != nil {
			slog.Error("Failed to read migration file", "file", file, "error", err)
			os.Exit(1)
		}

		parts := strings.Split(file, "_")
		var version int64
		if len(parts) > 0 {
			_, _ = fmt.Sscanf(parts[0], "%d", &version)
		}

		hash := sha256.Sum256(sqlBytes)
		checksumHex := hex.EncodeToString(hash[:])

		if version > 0 {
			var recordedChecksum string
			err := pool.QueryRow(ctx, "SELECT checksum FROM pcp_schema_migrations WHERE version = $1", version).Scan(&recordedChecksum)
			if err == nil {
				if recordedChecksum == checksumHex {
					slog.Info("Migration already applied, skipping", "version", version, "file", file, "checksum", checksumHex[:8])
					continue
				}
				slog.Error("FATAL: Migration checksum mismatch / drift detected! File has been altered after initial deployment",
					"version", version,
					"file", file,
					"recorded_checksum", recordedChecksum,
					"computed_checksum", checksumHex,
				)
				os.Exit(1)
			}
		}

		slog.Info("Executing migration", "file", file, "version", version, "checksum", checksumHex[:8])

		if _, err := pool.Exec(ctx, string(sqlBytes)); err != nil {
			slog.Error("Failed to execute migration", "file", file, "error", err)
			os.Exit(1)
		}

		if version > 0 {
			_, err = pool.Exec(ctx, `
				INSERT INTO pcp_schema_migrations (version, name, checksum)
				VALUES ($1, $2, $3)
			`, version, file, checksumHex)
			if err != nil {
				slog.Error("Failed to record migration in pcp_schema_migrations", "file", file, "error", err)
				os.Exit(1)
			}
		}
	}

	slog.Info("All database migrations applied successfully and verified in pcp_schema_migrations")
}
