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

func TestMigration000011_AgentKeyBindings(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("POSTGRES_URL")
	}
	if dbURL == "" {
		t.Fatalf("Failing migration integration test; no POSTGRES_URL provided")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to db: %v", err)
	}
	defer pool.Close()

	migrationsDir := filepath.Join("..", "..", "..", "db", "migrations")

	// Apply up to 000010 by running the previous test's setup logic
	// Or we just run migrations directly up to 000011
	// The runMigrations helper in this file currently does not include 000010 and 000011, let's fix that.
	runMigrations(t, ctx, pool, migrationsDir) // runMigrations only goes to 000009 currently

	m10Up, _ := os.ReadFile(filepath.Join(migrationsDir, "000010_agent_enrollment_keys.up.sql"))
	_, _ = pool.Exec(ctx, string(m10Up))

	m11Up, err := os.ReadFile(filepath.Join(migrationsDir, "000011_agent_key_bindings.up.sql"))
	if err != nil {
		t.Fatalf("failed to read 000011 up sql: %v", err)
	}
	m11Down, err := os.ReadFile(filepath.Join(migrationsDir, "000011_agent_key_bindings.down.sql"))
	if err != nil {
		t.Fatalf("failed to read 000011 down sql: %v", err)
	}

	// Make sure we start clean down
	_, _ = pool.Exec(ctx, string(m11Down))

	if _, err := pool.Exec(ctx, string(m11Up)); err != nil {
		t.Fatalf("failed to apply 000011 up: %v", err)
	}

	// Seed organizations and users correctly
	orgID1 := "org_mig11_1"
	orgID2 := "org_mig11_2"
	pool.Exec(ctx, "INSERT INTO organizations (organization_id, name, slug) VALUES ($1, 'Org 1', 'org-1') ON CONFLICT DO NOTHING", orgID1)
	pool.Exec(ctx, "INSERT INTO organizations (organization_id, name, slug) VALUES ($1, 'Org 2', 'org-2') ON CONFLICT DO NOTHING", orgID2)
	pool.Exec(ctx, "INSERT INTO users (user_id, email, password_hash, display_name) VALUES ('usr_mig11', 'm11@test.com', 'hash', 'Test') ON CONFLICT DO NOTHING")
	pool.Exec(ctx, "INSERT INTO agent_enrollment_keys (key_id, organization_id, created_by, name, token_prefix, token_hash) VALUES ('key_mig11', $1, 'usr_mig11', 'Test Key', 'cpk_', 'hash11') ON CONFLICT DO NOTHING", orgID1)
	
	// Create Devices
	pool.Exec(ctx, "INSERT INTO devices (device_id, organization_id, name, serial_number, model, platform_version, status, capabilities) VALUES ('dev1', $1, 'D1', 'SN1', 'M1', '1', 'active', '{}') ON CONFLICT DO NOTHING", orgID1)
	pool.Exec(ctx, "INSERT INTO devices (device_id, organization_id, name, serial_number, model, platform_version, status, capabilities) VALUES ('dev2', $1, 'D2', 'SN2', 'M2', '2', 'active', '{}') ON CONFLICT DO NOTHING", orgID2)

	t.Run("client_instance_id uniqueness per organization", func(t *testing.T) {
		_, err := pool.Exec(ctx, "INSERT INTO device_agents (agent_id, organization_id, device_id, client_instance_id, public_key_fingerprint, apk_version, protocol_version, status) VALUES ('agt1', $1, 'dev1', 'ci_1', '1111111111111111111111111111111111111111111111111111111111111111', '1.0', '2', 'active')", orgID1)
		if err != nil { t.Errorf("expected success: %v", err) }

		// Same org, same ci -> fail
		_, err = pool.Exec(ctx, "INSERT INTO device_agents (agent_id, organization_id, device_id, client_instance_id, public_key_fingerprint, apk_version, protocol_version, status) VALUES ('agt2', $1, 'dev1', 'ci_1', '2222222222222222222222222222222222222222222222222222222222222222', '1.0', '2', 'active')", orgID1)
		if err == nil { t.Errorf("expected unique constraint violation for client_instance_id") }

		// Different org, same ci -> success
		_, err = pool.Exec(ctx, "INSERT INTO device_agents (agent_id, organization_id, device_id, client_instance_id, public_key_fingerprint, apk_version, protocol_version, status) VALUES ('agt3', $1, 'dev2', 'ci_1', '3333333333333333333333333333333333333333333333333333333333333333', '1.0', '2', 'active')", orgID2)
		if err != nil { t.Errorf("expected success across different orgs: %v", err) }
	})

	t.Run("fingerprint lowercase 64-char CHECK", func(t *testing.T) {
		// Insert valid agent first
		pool.Exec(ctx, "INSERT INTO device_agents (agent_id, organization_id, device_id, client_instance_id, public_key_fingerprint, apk_version, protocol_version, status) VALUES ('agt4', $1, 'dev1', 'ci_4', '4444444444444444444444444444444444444444444444444444444444444444', '1.0', '2', 'active')", orgID1)
		
		// Uppercase
		_, err := pool.Exec(ctx, "INSERT INTO agent_key_bindings (binding_id, organization_id, key_id, device_id, agent_id, public_key_fingerprint) VALUES ('b_fail1', $1, 'key_mig11', 'dev1', 'agt4', '444444444444444444444444444444444444444444444444444444444444444A')", orgID1)
		if err == nil { t.Errorf("expected CHECK constraint failure for uppercase fingerprint") }
		
		// Too short
		_, err = pool.Exec(ctx, "INSERT INTO agent_key_bindings (binding_id, organization_id, key_id, device_id, agent_id, public_key_fingerprint) VALUES ('b_fail2', $1, 'key_mig11', 'dev1', 'agt4', '4444444444444444')", orgID1)
		if err == nil { t.Errorf("expected CHECK constraint failure for short fingerprint") }
	})

	t.Run("cross-tenant key FK rejected", func(t *testing.T) {
		_, err := pool.Exec(ctx, "INSERT INTO agent_key_bindings (binding_id, organization_id, key_id, device_id, agent_id, public_key_fingerprint) VALUES ('b1', $1, 'key_mig11', 'dev2', 'agt3', '3333333333333333333333333333333333333333333333333333333333333333')", orgID2)
		if err == nil { t.Errorf("expected FK rejection for cross tenant key") }
	})

	t.Run("active binding partial unique index", func(t *testing.T) {
		_, err := pool.Exec(ctx, "INSERT INTO agent_key_bindings (binding_id, organization_id, key_id, device_id, agent_id, public_key_fingerprint) VALUES ('b2', $1, 'key_mig11', 'dev1', 'agt1', '1111111111111111111111111111111111111111111111111111111111111111')", orgID1)
		if err != nil { t.Errorf("expected success: %v", err) }

		// Same agent, another active binding
		_, err = pool.Exec(ctx, "INSERT INTO agent_key_bindings (binding_id, organization_id, key_id, device_id, agent_id, public_key_fingerprint) VALUES ('b3', $1, 'key_mig11', 'dev1', 'agt1', '1111111111111111111111111111111111111111111111111111111111111111')", orgID1)
		if err == nil { t.Errorf("expected partial unique index violation for active binding") }

		// Release first one
		pool.Exec(ctx, "UPDATE agent_key_bindings SET released_at = NOW() WHERE binding_id = 'b2'")

		// Retry -> success
		_, err = pool.Exec(ctx, "INSERT INTO agent_key_bindings (binding_id, organization_id, key_id, device_id, agent_id, public_key_fingerprint) VALUES ('b3', $1, 'key_mig11', 'dev1', 'agt1', '1111111111111111111111111111111111111111111111111111111111111111')", orgID1)
		if err != nil { t.Errorf("expected success after releasing previous: %v", err) }
	})

	// Rollback 11
	if _, err := pool.Exec(ctx, string(m11Down)); err != nil {
		t.Fatalf("failed to apply 000011 down: %v", err)
	}

	// Re-apply 11
	if _, err := pool.Exec(ctx, string(m11Up)); err != nil {
		t.Fatalf("failed to re-apply 000011 up: %v", err)
	}
}

