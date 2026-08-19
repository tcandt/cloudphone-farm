package postgres

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigration000010_AgentEnrollmentKeys(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("POSTGRES_URL")
	}
	if dbURL == "" {
		t.Fatalf("Failing migration integration test; no POSTGRES_URL provided (must not be SKIP)")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	migrationsDir := filepath.Join("..", "..", "..", "db", "migrations")

	// Apply up to 000009
	runMigrations(t, ctx, pool, migrationsDir)

	m10Up, err := os.ReadFile(filepath.Join(migrationsDir, "000010_agent_enrollment_keys.up.sql"))
	if err != nil {
		t.Fatalf("failed to read 000010 up sql: %v", err)
	}
	m10Down, err := os.ReadFile(filepath.Join(migrationsDir, "000010_agent_enrollment_keys.down.sql"))
	if err != nil {
		t.Fatalf("failed to read 000010 down sql: %v", err)
	}

	// Make sure table does not exist yet (if ran before, we should drop it)
	_, _ = pool.Exec(ctx, string(m10Down))

	// Apply 10
	if _, err := pool.Exec(ctx, string(m10Up)); err != nil {
		t.Fatalf("failed to apply 000010 up: %v", err)
	}

	// Seed dependencies
	_, err = pool.Exec(ctx, "INSERT INTO organizations (organization_id, name, slug) VALUES ('org_migration_test', 'Test', 'test') ON CONFLICT DO NOTHING")
	if err != nil { t.Fatalf("failed to insert org: %v", err) }
	_, err = pool.Exec(ctx, "INSERT INTO users (user_id, email, password_hash, display_name) VALUES ('user_migration_test', 'test@test.com', 'hash', 'Test') ON CONFLICT DO NOTHING")
	if err != nil { t.Fatalf("failed to insert user: %v", err) }

	// Test constraints
	t.Run("max_bindings constraints", func(t *testing.T) {
		// max_bindings = NULL accepted
		_, err := pool.Exec(ctx, "INSERT INTO agent_enrollment_keys (key_id, organization_id, created_by, name, token_prefix, token_hash, max_bindings) VALUES ('k1', 'org_migration_test', 'user_migration_test', 'n1', 'cpk_1', 'hash1', NULL)")
		if err != nil {
			t.Errorf("expected max_bindings=NULL to be accepted, got err: %v", err)
		}

		// max_bindings > 0 accepted
		_, err = pool.Exec(ctx, "INSERT INTO agent_enrollment_keys (key_id, organization_id, created_by, name, token_prefix, token_hash, max_bindings) VALUES ('k2', 'org_migration_test', 'user_migration_test', 'n2', 'cpk_2', 'hash2', 10)")
		if err != nil {
			t.Errorf("expected max_bindings>0 to be accepted, got err: %v", err)
		}

		// max_bindings = 0 rejected
		_, err = pool.Exec(ctx, "INSERT INTO agent_enrollment_keys (key_id, organization_id, created_by, name, token_prefix, token_hash, max_bindings) VALUES ('k3', 'org_migration_test', 'user_migration_test', 'n3', 'cpk_3', 'hash3', 0)")
		if err == nil {
			t.Errorf("expected max_bindings=0 to be rejected")
		}

		// max_bindings < 0 rejected
		_, err = pool.Exec(ctx, "INSERT INTO agent_enrollment_keys (key_id, organization_id, created_by, name, token_prefix, token_hash, max_bindings) VALUES ('k4', 'org_migration_test', 'user_migration_test', 'n4', 'cpk_4', 'hash4', -5)")
		if err == nil {
			t.Errorf("expected max_bindings<0 to be rejected")
		}
	})

	t.Run("unique (organization_id, key_id) exists", func(t *testing.T) {
		_, err := pool.Exec(ctx, "INSERT INTO agent_enrollment_keys (key_id, organization_id, created_by, name, token_prefix, token_hash, max_bindings) VALUES ('k1', 'org_migration_test', 'user_migration_test', 'n5', 'cpk_5', 'hash5', NULL)")
		if err == nil {
			t.Errorf("expected unique constraint violation for (organization_id, key_id)")
		}
	})

	t.Run("foreign key rejections", func(t *testing.T) {
		// Unknown org
		_, err := pool.Exec(ctx, "INSERT INTO agent_enrollment_keys (key_id, organization_id, created_by, name, token_prefix, token_hash) VALUES ('k10', 'unknown_org', 'user_migration_test', 'n10', 'cpk_10', 'hash10')")
		if err == nil {
			t.Errorf("expected FK rejection for unknown org")
		}

		// Unknown user
		_, err = pool.Exec(ctx, "INSERT INTO agent_enrollment_keys (key_id, organization_id, created_by, name, token_prefix, token_hash) VALUES ('k11', 'org_migration_test', 'unknown_user', 'n11', 'cpk_11', 'hash11')")
		if err == nil {
			t.Errorf("expected FK rejection for unknown user")
		}
	})

	// Rollback 10
	if _, err := pool.Exec(ctx, string(m10Down)); err != nil {
		t.Fatalf("failed to apply 000010 down: %v", err)
	}

	// Verify table removed
	var exists bool
	err = pool.QueryRow(ctx, "SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'agent_enrollment_keys')").Scan(&exists)
	if err != nil || exists {
		t.Fatalf("expected table to be removed, exists=%v, err=%v", exists, err)
	}

	// Re-apply 10
	if _, err := pool.Exec(ctx, string(m10Up)); err != nil {
		t.Fatalf("failed to re-apply 000010 up: %v", err)
	}
}
