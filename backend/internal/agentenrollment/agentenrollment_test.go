package agentenrollment_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentenrollment"
	"github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)

var dbMigrationMutex sync.Mutex

func runMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool, migrationsDir string) {
	dbMigrationMutex.Lock()
	defer dbMigrationMutex.Unlock()

	_, _ = pool.Exec(ctx, "SELECT pg_advisory_lock(123456789);")
	defer func() {
		_, _ = pool.Exec(ctx, "SELECT pg_advisory_unlock(123456789);")
	}()

	migrations := []string{
		"000001_create_core_tables.up.sql",
		"000002_seed_initial_rbac.up.sql",
		"000003_harden_agent_identity_and_enrollment.up.sql",
		"000004_harden_command_outbox.up.sql",
		"000005_harden_command_runtime.up.sql",
		"000006_control_lease_and_command_contract.up.sql",
		"000007_phase14_command_delivery_attempts.up.sql",
		"000008_nullable_physical_telemetry_and_security_metadata.up.sql",
		"000009_enrollment_tokens_created_at.up.sql",
		"000010_agent_enrollment_keys.up.sql",
		"000011_agent_key_bindings.up.sql",
	}

	_, _ = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS pcp_schema_migrations (
			version BIGINT PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			checksum VARCHAR(64) NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)

	for _, mFile := range migrations {
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
		_ = tx.Commit(ctx)
	}
}

type ecdsaSignature struct {
	R, S *big.Int
}

func TestEnrollmentV2_Integration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	redisURL := os.Getenv("REDIS_URL")
	if dbURL == "" || redisURL == "" {
		t.Skip("Skipping integration test; DATABASE_URL or REDIS_URL not set")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to DB: %v", err)
	}
	defer pool.Close()

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		t.Fatalf("failed to parse redis url: %v", err)
	}
	rdb := redis.NewClient(opt)
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("failed to ping redis: %v", err)
	}

	wd, _ := os.Getwd()
	migrationsDir := filepath.Join(wd, "..", "..", "db", "migrations")
	if strings.Contains(wd, "agentenrollment") {
		migrationsDir = filepath.Join(wd, "..", "..", "..", "db", "migrations")
	}
	runMigrations(t, ctx, pool, migrationsDir)

	repo := postgres.NewEnrollmentV2Repository(pool)
	challengeStore := agentenrollment.NewChallengeStore(rdb)
	service := agentenrollment.NewEnrollmentV2Service(repo, challengeStore)

	// Seed data
	orgID := "org_v2_test"
	userID := "user_v2_test"
	pool.Exec(ctx, "INSERT INTO organizations (organization_id, name) VALUES ($1, $2) ON CONFLICT DO NOTHING", orgID, "Test Org V2")
	pool.Exec(ctx, "INSERT INTO users (user_id, email, password_hash, organization_id, role) VALUES ($1, $2, $3, $4, $5) ON CONFLICT DO NOTHING", userID, "v2test@example.com", "hash", orgID, "admin")

	keyID := "key_test_1"
	rawSecret := "secret_12345"
	tokenPrefix := "tc_"
	tokenHash := crypto.HashToken(rawSecret)

	// Create an enrollment key with max_bindings = 1
	_, err = pool.Exec(ctx, `
		INSERT INTO agent_enrollment_keys (key_id, organization_id, created_by, name, token_hash, token_prefix, max_bindings)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (key_id) DO UPDATE SET max_bindings = 1
	`, keyID, orgID, userID, "V2 Test Key", tokenHash, tokenPrefix, 1)
	if err != nil {
		t.Fatalf("failed to seed key: %v", err)
	}

	// Helper to generate a new keypair
	genKey := func() (string, *ecdsa.PrivateKey) {
		priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		pubBytes, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		return base64.StdEncoding.EncodeToString(pubBytes), priv
	}

	signNonce := func(nonceB64url string, priv *ecdsa.PrivateKey) string {
		nonceRaw, _ := base64.RawURLEncoding.DecodeString(nonceB64url)
		hash := sha256.Sum256(nonceRaw)
		r, s, _ := ecdsa.Sign(rand.Reader, priv, hash[:])
		sigBytes, _ := asn1.Marshal(ecdsaSignature{R: r, S: s})
		return base64.StdEncoding.EncodeToString(sigBytes)
	}

	// 1. Success path
	pubKeyBase64, privKey := genKey()
	clientInst1 := "ci_1"

	challengeID1, nonce1, err := service.GenerateChallenge(ctx, rawSecret, clientInst1, pubKeyBase64)
	if err != nil {
		t.Fatalf("GenerateChallenge failed: %v", err)
	}

	sig1 := signNonce(nonce1, privKey)
	req1 := agentenrollment.AgentEnrollRequest{
		EnrollmentToken:  rawSecret,
		ChallengeID:      challengeID1,
		ClientInstanceID: clientInst1,
		PublicKey:        pubKeyBase64,
		Signature:        sig1,
		DeviceInfo: map[string]interface{}{
			"serial_number": "SN001",
		},
	}

	res1, created, err := service.EnrollAgent(ctx, req1)
	if err != nil {
		t.Fatalf("EnrollAgent failed: %v", err)
	}
	if !created {
		t.Errorf("expected created=true for new identity")
	}
	if res1.AgentID == "" || res1.DeviceID == "" {
		t.Errorf("expected non-empty IDs")
	}

	// 2. Idempotent success (same identity)
	challengeID2, nonce2, _ := service.GenerateChallenge(ctx, rawSecret, clientInst1, pubKeyBase64)
	sig2 := signNonce(nonce2, privKey)
	req2 := req1
	req2.ChallengeID = challengeID2
	req2.Signature = sig2

	res2, created2, err := service.EnrollAgent(ctx, req2)
	if err != nil {
		t.Fatalf("Idempotent enroll failed: %v", err)
	}
	if created2 {
		t.Errorf("expected created=false for idempotent request")
	}
	if res2.AgentID != res1.AgentID {
		t.Errorf("expected same agent ID")
	}

	// 3. Different identity, Quota exhaustion (max_bindings = 1)
	pubKeyBase64_2, privKey2 := genKey()
	clientInst2 := "ci_2"
	challengeID3, nonce3, _ := service.GenerateChallenge(ctx, rawSecret, clientInst2, pubKeyBase64_2)
	sig3 := signNonce(nonce3, privKey2)

	req3 := req1
	req3.ChallengeID = challengeID3
	req3.ClientInstanceID = clientInst2
	req3.PublicKey = pubKeyBase64_2
	req3.Signature = sig3

	_, _, err = service.EnrollAgent(ctx, req3)
	if err != agentenrollment.ErrQuotaExhausted {
		t.Errorf("expected ErrQuotaExhausted, got: %v", err)
	}

	// 4. Same ClientInstanceID, Different PublicKey -> Identity Conflict
	pubKeyBase64_3, privKey3 := genKey()
	challengeID4, nonce4, _ := service.GenerateChallenge(ctx, rawSecret, clientInst1, pubKeyBase64_3)
	sig4 := signNonce(nonce4, privKey3)
	
	req4 := req1
	req4.ChallengeID = challengeID4
	req4.PublicKey = pubKeyBase64_3
	req4.Signature = sig4
	
	_, _, err = service.EnrollAgent(ctx, req4)
	if err != agentenrollment.ErrIdentityConflict {
		t.Errorf("expected ErrIdentityConflict, got: %v", err)
	}
	
	// Clean up for other tests
	pool.Exec(ctx, "DELETE FROM agent_key_bindings WHERE key_id = $1", keyID)
	pool.Exec(ctx, "DELETE FROM agent_enrollment_keys WHERE key_id = $1", keyID)
}
