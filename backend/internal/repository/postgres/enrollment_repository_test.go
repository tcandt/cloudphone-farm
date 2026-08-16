package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRecordDeviceHeartbeat_NilPoolNoPanics(t *testing.T) {
	repo := NewEnrollmentRepository(nil)

	err := repo.RecordDeviceHeartbeat(context.Background(), "org_1", "dev_1", nil, nil, nil, nil, nil, []byte(`{"security_level":"STRONGBOX"}`))
	if err != nil {
		t.Fatalf("expected nil error on nil pool, got: %v", err)
	}
}

func TestPostgreSQLRecordDeviceHeartbeat_NullableTelemetryAndKeyProtection(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("POSTGRES_URL")
	}
	if dbURL == "" {
		t.Skip("Skipping PostgreSQL integration test: DATABASE_URL and POSTGRES_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	// Apply production SQL migrations
	migrations := []string{
		"000001_create_core_tables.up.sql",
		"000002_seed_initial_rbac.up.sql",
		"000003_harden_agent_identity_and_enrollment.up.sql",
		"000004_harden_command_outbox.up.sql",
		"000005_harden_command_runtime.up.sql",
		"000006_control_lease_and_command_contract.up.sql",
		"000007_phase14_command_delivery_attempts.up.sql",
		"000008_nullable_physical_telemetry_and_security_metadata.up.sql",
	}

	// Ensure pcp_schema_migrations table exists
	_, _ = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS pcp_schema_migrations (
			version BIGINT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			checksum VARCHAR(64) NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)

	migrationsDir := filepath.Join("..", "..", "..", "db", "migrations")
	for _, mFile := range migrations {
		var version int64
		if parts := strings.Split(mFile, "_"); len(parts) > 0 {
			_, _ = fmt.Sscanf(parts[0], "%d", &version)
		}

		if version > 0 {
			var recordedChecksum string
			err := pool.QueryRow(ctx, "SELECT checksum FROM pcp_schema_migrations WHERE version = $1", version).Scan(&recordedChecksum)
			if err == nil {
				// Already applied by production migrator or prior step
				continue
			}
		}

		mPath := filepath.Join(migrationsDir, mFile)
		sqlBytes, readErr := os.ReadFile(mPath)
		if readErr != nil {
			t.Fatalf("failed to read migration file %s: %v", mPath, readErr)
		}

		tx, txErr := pool.Begin(ctx)
		if txErr != nil {
			t.Fatalf("failed to begin migration transaction for %s: %v", mFile, txErr)
		}
		if _, execErr := tx.Exec(ctx, string(sqlBytes)); execErr != nil {
			_ = tx.Rollback(ctx)
			t.Fatalf("failed to execute migration %s: %v", mFile, execErr)
		}
		if version > 0 {
			_, _ = tx.Exec(ctx, "INSERT INTO pcp_schema_migrations (version, name, checksum) VALUES ($1, $2, 'test_checksum') ON CONFLICT DO NOTHING", version, mFile)
		}
		_ = tx.Commit(ctx)
	}

	orgID := "org_test_hb_sql"
	deviceID := "dev_test_hb_001"

	// Cleanup
	_, _ = pool.Exec(ctx, "DELETE FROM device_heartbeats WHERE organization_id = $1", orgID)
	_, _ = pool.Exec(ctx, "DELETE FROM device_agents WHERE organization_id = $1", orgID)
	_, _ = pool.Exec(ctx, "DELETE FROM devices WHERE organization_id = $1", orgID)
	_, _ = pool.Exec(ctx, "DELETE FROM organizations WHERE organization_id = $1", orgID)

	// Insert production seed hierarchy matching 000001_create_core_tables.up.sql
	_, err = pool.Exec(ctx, `
		INSERT INTO organizations (organization_id, name, slug)
		VALUES ($1, 'HB Test Org', 'org-test-hb-sql')
	`, orgID)
	if err != nil {
		t.Fatalf("failed to insert organization: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO devices (device_id, organization_id, name, serial_number, model, platform_version, status)
		VALUES ($1, $2, 'Test Device', 'sn_hb_001', 'Pixel 6', '14.0', 'ENROLLED')
	`, deviceID, orgID)
	if err != nil {
		t.Fatalf("failed to insert device: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO device_agents (agent_id, organization_id, device_id, apk_version, protocol_version, status, registered_at)
		VALUES ($1, $2, $1, '1.0.0', 'v1', 'active', CURRENT_TIMESTAMP)
	`, deviceID, orgID)
	if err != nil {
		t.Fatalf("failed to insert device_agent: %v", err)
	}

	repo := NewEnrollmentRepository(pool)
	keyProtectionJSON := []byte(`{"algorithm":"AES-256-GCM","provider":"AndroidKeyStore","security_level":"STRONGBOX"}`)

	// 1. Invoke RecordDeviceHeartbeat with nil telemetry pointers
	err = repo.RecordDeviceHeartbeat(ctx, orgID, deviceID, nil, nil, nil, nil, nil, keyProtectionJSON)
	if err != nil {
		t.Fatalf("RecordDeviceHeartbeat failed: %v", err)
	}

	// 2. Query device_heartbeats and assert telemetry columns are SQL NULL
	var cpu, ram, temp *float64
	var battery *int
	var network *string

	row := pool.QueryRow(ctx, `
		SELECT cpu_usage, memory_usage, battery_level, temperature_c, network_type
		FROM device_heartbeats
		WHERE organization_id = $1 AND device_id = $2
		ORDER BY received_at DESC LIMIT 1
	`, orgID, deviceID)

	if err := row.Scan(&cpu, &ram, &battery, &temp, &network); err != nil {
		t.Fatalf("failed to scan device_heartbeats row: %v", err)
	}

	if cpu != nil {
		t.Errorf("expected cpu_usage to be SQL NULL, got: %v", *cpu)
	}
	if ram != nil {
		t.Errorf("expected memory_usage to be SQL NULL, got: %v", *ram)
	}
	if battery != nil {
		t.Errorf("expected battery_level to be SQL NULL, got: %v", *battery)
	}
	if temp != nil {
		t.Errorf("expected temperature_c to be SQL NULL, got: %v", *temp)
	}
	if network != nil {
		t.Errorf("expected network_type to be SQL NULL, got: %v", *network)
	}

	// 3. Query device_agents & devices key_protection columns and assert exact JSON value
	var daKeyProtRaw []byte
	err = pool.QueryRow(ctx, `SELECT key_protection FROM device_agents WHERE organization_id = $1 AND device_id = $2`, orgID, deviceID).Scan(&daKeyProtRaw)
	if err != nil {
		t.Fatalf("failed to query device_agents key_protection: %v", err)
	}

	var daMap map[string]interface{}
	if err := json.Unmarshal(daKeyProtRaw, &daMap); err != nil {
		t.Fatalf("failed to unmarshal device_agents key_protection: %v", err)
	}
	if daMap["security_level"] != "STRONGBOX" {
		t.Errorf("expected security_level STRONGBOX on device_agents, got: %v", daMap["security_level"])
	}

	var devKeyProtRaw []byte
	err = pool.QueryRow(ctx, `SELECT key_protection FROM devices WHERE organization_id = $1 AND device_id = $2`, orgID, deviceID).Scan(&devKeyProtRaw)
	if err != nil {
		t.Fatalf("failed to query devices key_protection: %v", err)
	}

	var devMap map[string]interface{}
	if err := json.Unmarshal(devKeyProtRaw, &devMap); err != nil {
		t.Fatalf("failed to unmarshal devices key_protection: %v", err)
	}
	if devMap["security_level"] != "STRONGBOX" {
		t.Errorf("expected security_level STRONGBOX on devices, got: %v", devMap["security_level"])
	}

	// 4. Assert error propagation when key_protection UPDATE fails (invalid JSONB syntax)
	invalidJSON := []byte("INVALID_JSON_SYNTAX")
	err = repo.RecordDeviceHeartbeat(ctx, orgID, deviceID, nil, nil, nil, nil, nil, invalidJSON)
	if err == nil {
		t.Fatalf("expected error when updating key_protection with invalid JSONB, got nil")
	}
}
