package postgres_test

import (
	"os"
	"strings"
	"testing"
)

func TestMigrationSQLFilesValidity(t *testing.T) {
	upSQL, err := os.ReadFile("../../../db/migrations/000001_create_core_tables.up.sql")
	if err != nil {
		t.Fatalf("Failed to read 000001_create_core_tables.up.sql: %v", err)
	}

	if len(upSQL) == 0 {
		t.Fatal("Migration up file 000001_create_core_tables.up.sql is empty")
	}

	downSQL, err := os.ReadFile("../../../db/migrations/000001_create_core_tables.down.sql")
	if err != nil {
		t.Fatalf("Failed to read 000001_create_core_tables.down.sql: %v", err)
	}

	if len(downSQL) == 0 {
		t.Fatal("Migration down file 000001_create_core_tables.down.sql is empty")
	}

	// Verify all 19 tables are present in the up migration
	expectedTables := []string{
		"organizations",
		"users",
		"organization_memberships",
		"roles",
		"permissions",
		"role_permissions",
		"user_roles",
		"devices",
		"device_agents",
		"device_heartbeats",
		"control_leases",
		"commands",
		"command_events",
		"command_outbox",
		"proxies",
		"proxy_assignments",
		"enrollment_tokens",
		"sessions",
		"audit_logs",
	}

	upContent := string(upSQL)
	for _, tbl := range expectedTables {
		if !strings.Contains(upContent, "CREATE TABLE IF NOT EXISTS "+tbl) {
			t.Errorf("Migration up SQL missing expected table: %s", tbl)
		}
	}

	// Verify security constraints
	if !strings.Contains(upContent, "token_hash") {
		t.Error("Enrollment tokens must use token_hash, raw tokens forbidden")
	}
	if !strings.Contains(upContent, "ciphertext BYTEA") || !strings.Contains(upContent, "nonce BYTEA") {
		t.Error("Proxies table must use ciphertext BYTEA and nonce BYTEA for at-rest encryption")
	}
	if !strings.Contains(upContent, "CONSTRAINT uk_org_device UNIQUE (organization_id, device_id)") {
		t.Error("Devices table missing composite tenant unique constraint (organization_id, device_id)")
	}
}
