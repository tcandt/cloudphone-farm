package crypto_test

import (
	"os"
	"strings"
	"testing"

	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)

func TestMigrationSQLFilesValidity(t *testing.T) {
	upSQL, err := os.ReadFile("../../db/migrations/000001_create_core_tables.up.sql")
	if err != nil {
		t.Fatalf("Failed to read 000001_create_core_tables.up.sql: %v", err)
	}

	if len(upSQL) == 0 {
		t.Fatal("Migration up file 000001_create_core_tables.up.sql is empty")
	}

	downSQL, err := os.ReadFile("../../db/migrations/000001_create_core_tables.down.sql")
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

	// Verify Migration 000003
	m3SQL, err := os.ReadFile("../../db/migrations/000003_harden_agent_identity_and_enrollment.up.sql")
	if err != nil {
		t.Fatalf("Failed to read 000003_harden_agent_identity_and_enrollment.up.sql: %v", err)
	}
	m3Content := string(m3SQL)
	if !strings.Contains(m3Content, "public_key_fingerprint") || !strings.Contains(m3Content, "consumed_at") {
		t.Error("Migration 000003 missing required public_key_fingerprint or consumed_at columns")
	}

	// Verify Migration 000004
	m4SQL, err := os.ReadFile("../../db/migrations/000004_harden_command_outbox.up.sql")
	if err != nil {
		t.Fatalf("Failed to read 000004_harden_command_outbox.up.sql: %v", err)
	}
	m4Content := string(m4SQL)
	if !strings.Contains(m4Content, "attempt_count") || !strings.Contains(m4Content, "dispatched_at") {
		t.Error("Migration 000004 missing required attempt_count or dispatched_at columns")
	}
}

func TestArgon2idPasswordHashingAndVerification(t *testing.T) {
	password := "Pcp_Secure_Pass_2026!#"

	hash, err := crypto.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	if len(hash) == 0 {
		t.Fatal("HashPassword returned empty string")
	}

	// Verify valid password
	match, err := crypto.VerifyPassword(password, hash)
	if err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
	if !match {
		t.Errorf("Expected password verification to succeed, got false")
	}

	// Verify wrong password
	matchWrong, err := crypto.VerifyPassword("WrongPassword123!", hash)
	if err != nil {
		t.Fatalf("VerifyPassword error on wrong password: %v", err)
	}
	if matchWrong {
		t.Errorf("Expected wrong password verification to fail, got true")
	}

	// Verify malformed hash handling (must not panic!)
	malformedHash := "$argon2id$v=19$m=65536,t=3,p=4$invalid_salt$invalid_hash"
	matchMalformed, err := crypto.VerifyPassword(password, malformedHash)
	if err == nil {
		t.Errorf("Expected error on malformed hash, got nil")
	}
	if matchMalformed {
		t.Errorf("Expected malformed hash verification to fail, got true")
	}
}

func TestOpaqueTokenGenerationAndHashing(t *testing.T) {
	token1, err := crypto.GenerateOpaqueToken()
	if err != nil {
		t.Fatalf("GenerateOpaqueToken failed: %v", err)
	}

	token2, err := crypto.GenerateOpaqueToken()
	if err != nil {
		t.Fatalf("GenerateOpaqueToken failed: %v", err)
	}

	if token1 == token2 {
		t.Errorf("GenerateOpaqueToken produced identical tokens")
	}

	hash1 := crypto.HashToken(token1)
	hash1Again := crypto.HashToken(token1)
	hash2 := crypto.HashToken(token2)

	if hash1 != hash1Again {
		t.Errorf("HashToken is not deterministic")
	}

	if hash1 == hash2 {
		t.Errorf("HashToken produced collision for different tokens")
	}

	if len(hash1) != 64 {
		t.Errorf("Expected SHA-256 hex length 64, got %d", len(hash1))
	}
}

func TestEd25519AgentKeyPairAndFingerprint(t *testing.T) {
	pubB64, privB64, fp, err := crypto.GenerateEd25519KeyPair()
	if err != nil {
		t.Fatalf("GenerateEd25519KeyPair failed: %v", err)
	}

	if pubB64 == "" || privB64 == "" || fp == "" {
		t.Errorf("Generated keypair returned empty values")
	}

	if len(fp) != 64 {
		t.Errorf("Expected SHA-256 fingerprint hex length 64, got %d", len(fp))
	}
}
