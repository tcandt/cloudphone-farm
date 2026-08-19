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
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentenrollment"
	"github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)



type ecdsaSignature struct {
	R, S *big.Int
}

func TestEnrollmentV2_Integration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	redisURL := os.Getenv("REDIS_URL")
	if dbURL == "" || redisURL == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("Gate-critical integration test failed: CI=true but DATABASE_URL or REDIS_URL not set")
		}
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

	// assume DB is already migrated

	repo := postgres.NewEnrollmentV2Repository(pool)
	challengeStore := agentenrollment.NewChallengeStore(rdb)
	service := agentenrollment.NewEnrollmentV2Service(repo, challengeStore)

	// Seed data
	orgID := "org_v2_test"
	userID := "user_v2_test"
	pool.Exec(ctx, "INSERT INTO organizations (organization_id, name, slug) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING", orgID, "Test Org V2", "test-org-v2")
	pool.Exec(ctx, "INSERT INTO users (user_id, email, password_hash, display_name) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING", userID, "v2test@example.com", "hash", "Test User V2")

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
		DeviceInfo: agentenrollment.AgentDeviceInfo{
			Manufacturer:    "Google",
			Model:           "Pixel 6",
			AndroidVersion:  "12",
			SDKInt:          31,
			SerialNumber:    "SN001",
			AgentVersion:    "1.0.0",
			ProtocolVersion: "2",
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
	
	t.Run("TestEnrollmentV2_Concurrency_DifferentIdentitiesOneSlot", func(t *testing.T) {
		pool.Exec(ctx, "DELETE FROM agent_key_bindings WHERE key_id = $1", keyID)
		
		pubA1, privA1 := genKey()
		ciA1 := "ci_conc_a1"
		chalA1, nA1, _ := service.GenerateChallenge(ctx, rawSecret, ciA1, pubA1)
		reqA1 := req1
		reqA1.ClientInstanceID = ciA1
		reqA1.ChallengeID = chalA1
		reqA1.PublicKey = pubA1
		reqA1.Signature = signNonce(nA1, privA1)

		pubA2, privA2 := genKey()
		ciA2 := "ci_conc_a2"
		chalA2, nA2, _ := service.GenerateChallenge(ctx, rawSecret, ciA2, pubA2)
		reqA2 := req1
		reqA2.ClientInstanceID = ciA2
		reqA2.ChallengeID = chalA2
		reqA2.PublicKey = pubA2
		reqA2.Signature = signNonce(nA2, privA2)

		var wg sync.WaitGroup
		var errA1, errA2 error
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, _, errA1 = service.EnrollAgent(ctx, reqA1)
		}()
		go func() {
			defer wg.Done()
			_, _, errA2 = service.EnrollAgent(ctx, reqA2)
		}()
		wg.Wait()

		if errA1 == agentenrollment.ErrQuotaExhausted && errA2 == nil {
			// A2 succeeded, A1 exhausted
		} else if errA2 == agentenrollment.ErrQuotaExhausted && errA1 == nil {
			// A1 succeeded, A2 exhausted
		} else {
			t.Errorf("Concurrency A failed: expected exactly one success and one ErrQuotaExhausted, got errA1: %v, errA2: %v", errA1, errA2)
		}

		var count int
		err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM agent_key_bindings WHERE key_id = $1 AND released_at IS NULL", keyID).Scan(&count)
		if err != nil || count != 1 {
			t.Errorf("expected exactly 1 active binding, got %d", count)
		}
	})

	t.Run("TestEnrollmentV2_Concurrency_SameIdentity", func(t *testing.T) {
		pool.Exec(ctx, "DELETE FROM agent_key_bindings WHERE key_id = $1", keyID)
		pool.Exec(ctx, "DELETE FROM device_agents WHERE organization_id = $1", orgID)

		pubB, privB := genKey()
		ciB := "ci_conc_b"
		
		chalB1, nB1, _ := service.GenerateChallenge(ctx, rawSecret, ciB, pubB)
		reqB1 := req1
		reqB1.ClientInstanceID = ciB
		reqB1.ChallengeID = chalB1
		reqB1.PublicKey = pubB
		reqB1.Signature = signNonce(nB1, privB)

		chalB2, nB2, _ := service.GenerateChallenge(ctx, rawSecret, ciB, pubB)
		reqB2 := req1
		reqB2.ClientInstanceID = ciB
		reqB2.ChallengeID = chalB2
		reqB2.PublicKey = pubB
		reqB2.Signature = signNonce(nB2, privB)

		var resB1, resB2 *agentenrollment.AgentEnrollResponse
		var errB1, errB2 error
		var createdB1, createdB2 bool
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			resB1, createdB1, errB1 = service.EnrollAgent(ctx, reqB1)
		}()
		go func() {
			defer wg.Done()
			resB2, createdB2, errB2 = service.EnrollAgent(ctx, reqB2)
		}()
		wg.Wait()

		if errB1 != nil || errB2 != nil {
			t.Errorf("Concurrency B failed: expected both to succeed (one created, one idempotent). errB1: %v, errB2: %v", errB1, errB2)
		} else {
			if createdB1 == createdB2 {
				t.Errorf("Concurrency B failed: one must be created=true and one created=false")
			}
			if resB1.AgentID != resB2.AgentID || resB1.DeviceID != resB2.DeviceID {
				t.Errorf("Concurrency B failed: expected same agent and device IDs")
			}
		}

		var countAgents int
		_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM device_agents WHERE client_instance_id = $1", ciB).Scan(&countAgents)
		if countAgents != 1 {
			t.Errorf("expected 1 agent, got %d", countAgents)
		}

		var countBindings int
		var agentID string
		if resB1 != nil {
			agentID = resB1.AgentID
		}
		_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM agent_key_bindings WHERE agent_id = $1 AND released_at IS NULL", agentID).Scan(&countBindings)
		if countBindings != 1 {
			t.Errorf("expected 1 binding, got %d", countBindings)
		}
	})

	// Clean up for other tests
	pool.Exec(ctx, "DELETE FROM agent_key_bindings WHERE key_id = $1", keyID)
	pool.Exec(ctx, "DELETE FROM agent_enrollment_keys WHERE key_id = $1", keyID)
}
