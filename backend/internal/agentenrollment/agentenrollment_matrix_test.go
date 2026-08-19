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
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentenrollment"
	"github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)

func TestEnrollmentV2_SecurityMatrix(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	redisURL := os.Getenv("REDIS_URL")
	if dbURL == "" || redisURL == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("Gate-critical integration test failed: CI=true but DATABASE_URL or REDIS_URL not set")
		}
		t.Skip("Skipping security matrix test; DATABASE_URL or REDIS_URL not set")
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

	repo := postgres.NewEnrollmentV2Repository(pool)
	challengeStore := agentenrollment.NewChallengeStore(rdb)
	service := agentenrollment.NewEnrollmentV2Service(repo, challengeStore)

	orgID := "org_v2_matrix"
	userID := "user_v2_matrix"
	pool.Exec(ctx, "INSERT INTO organizations (organization_id, name, slug) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING", orgID, "Test Org Matrix", "test-org-matrix")
	pool.Exec(ctx, "INSERT INTO users (user_id, email, password_hash, display_name) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING", userID, "matrix@example.com", "hash", "Test User")

	keyID := "key_matrix_1"
	rawSecret := "secret_matrix"
	tokenPrefix := "tc_"
	tokenHash := crypto.HashToken(rawSecret)

	setupKey := func(maxBindings *int, expiresAt *time.Time, revokedAt *time.Time) {
		pool.Exec(ctx, "DELETE FROM agent_key_bindings WHERE key_id = $1", keyID)
		pool.Exec(ctx, "DELETE FROM agent_enrollment_keys WHERE key_id = $1", keyID)
		pool.Exec(ctx, "DELETE FROM device_agents WHERE organization_id = $1", orgID)
		_, err := pool.Exec(ctx, `
			INSERT INTO agent_enrollment_keys (key_id, organization_id, created_by, name, token_hash, token_prefix, max_bindings, expires_at, revoked_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`, keyID, orgID, userID, "Matrix Key", tokenHash, tokenPrefix, maxBindings, expiresAt, revokedAt)
		if err != nil {
			t.Fatalf("failed to insert key: %v", err)
		}
	}

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

	validDeviceInfo := agentenrollment.AgentDeviceInfo{
		Manufacturer:    "Google",
		Model:           "Pixel 6",
		AndroidVersion:  "12",
		SDKInt:          31,
		SerialNumber:    "SN001",
		AgentVersion:    "1.0.0",
		ProtocolVersion: "2",
	}

	t.Run("invalid enrollment token", func(t *testing.T) {
		setupKey(nil, nil, nil)
		pubBase64, _ := genKey()
		_, _, err := service.GenerateChallenge(ctx, "invalid_secret", "ci_1", pubBase64)
		if err != agentenrollment.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("revoked enrollment key", func(t *testing.T) {
		now := time.Now()
		setupKey(nil, nil, &now)
		pubBase64, _ := genKey()
		_, _, err := service.GenerateChallenge(ctx, rawSecret, "ci_1", pubBase64)
		if err != agentenrollment.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("expired enrollment key", func(t *testing.T) {
		past := time.Now().Add(-1 * time.Hour)
		setupKey(nil, &past, nil)
		pubBase64, _ := genKey()
		_, _, err := service.GenerateChallenge(ctx, rawSecret, "ci_1", pubBase64)
		if err != agentenrollment.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("invalid Base64 public key", func(t *testing.T) {
		setupKey(nil, nil, nil)
		_, _, err := service.GenerateChallenge(ctx, rawSecret, "ci_1", "invalid_base64")
		if err == nil {
			t.Errorf("expected error for invalid base64")
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		setupKey(nil, nil, nil)
		pubBase64, priv := genKey()
		chalID, _, _ := service.GenerateChallenge(ctx, rawSecret, "ci_1", pubBase64)
		
		// Sign different data
		wrongSig := signNonce(base64.RawURLEncoding.EncodeToString([]byte("wrong")), priv)
		
		req := agentenrollment.AgentEnrollRequest{
			EnrollmentToken:  rawSecret,
			ChallengeID:      chalID,
			ClientInstanceID: "ci_1",
			PublicKey:        pubBase64,
			Signature:        wrongSig,
			DeviceInfo:       validDeviceInfo,
		}
		_, _, err := service.EnrollAgent(ctx, req)
		if err != agentenrollment.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("challenge replay", func(t *testing.T) {
		setupKey(nil, nil, nil)
		pubBase64, priv := genKey()
		chalID, nonce, _ := service.GenerateChallenge(ctx, rawSecret, "ci_1", pubBase64)
		sig := signNonce(nonce, priv)
		
		req := agentenrollment.AgentEnrollRequest{
			EnrollmentToken:  rawSecret,
			ChallengeID:      chalID,
			ClientInstanceID: "ci_1",
			PublicKey:        pubBase64,
			Signature:        sig,
			DeviceInfo:       validDeviceInfo,
		}
		_, _, err := service.EnrollAgent(ctx, req)
		if err != nil {
			t.Errorf("expected success on first enroll, got %v", err)
		}

		// Replay
		_, _, err = service.EnrollAgent(ctx, req)
		if err != agentenrollment.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized on replay, got %v", err)
		}
	})

	t.Run("unlimited max_bindings=NULL", func(t *testing.T) {
		setupKey(nil, nil, nil)
		for i := 0; i < 3; i++ {
			pub, priv := genKey()
			ci := fmt.Sprintf("ci_unlim_%d", i)
			chalID, nonce, _ := service.GenerateChallenge(ctx, rawSecret, ci, pub)
			req := agentenrollment.AgentEnrollRequest{
				EnrollmentToken:  rawSecret,
				ChallengeID:      chalID,
				ClientInstanceID: ci,
				PublicKey:        pub,
				Signature:        signNonce(nonce, priv),
				DeviceInfo:       validDeviceInfo,
			}
			_, _, err := service.EnrollAgent(ctx, req)
			if err != nil {
				t.Fatalf("expected success, got %v", err)
			}
		}
	})

	t.Run("PUBLIC KEY PERSISTENCE", func(t *testing.T) {
		setupKey(nil, nil, nil)
		pub, priv := genKey()
		ci := "ci_pub_persist"
		chalID, nonce, _ := service.GenerateChallenge(ctx, rawSecret, ci, pub)
		req := agentenrollment.AgentEnrollRequest{
			EnrollmentToken:  rawSecret,
			ChallengeID:      chalID,
			ClientInstanceID: ci,
			PublicKey:        pub,
			Signature:        signNonce(nonce, priv),
			DeviceInfo:       validDeviceInfo,
		}
		res, _, err := service.EnrollAgent(ctx, req)
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}

		var dbPub []byte
		var dbFp string
		err = pool.QueryRow(ctx, "SELECT public_key, public_key_fingerprint FROM device_agents WHERE agent_id = $1", res.AgentID).Scan(&dbPub, &dbFp)
		if err != nil {
			t.Fatalf("failed to query device_agents: %v", err)
		}

		// Assert lowercaseHex(SHA256(public_key)) == public_key_fingerprint
		hash := sha256.Sum256(dbPub)
		expectedFp := hex.EncodeToString(hash[:])
		if expectedFp != dbFp {
			t.Errorf("expected FP %s, got %s", expectedFp, dbFp)
		}
	})

	t.Run("SECRET HYGIENE", func(t *testing.T) {
		setupKey(nil, nil, nil)
		pub, priv := genKey()
		ci := "ci_secret_hygiene"
		chalID, nonce, _ := service.GenerateChallenge(ctx, rawSecret, ci, pub)
		
		val, err := rdb.Get(ctx, "agent:enroll:challenge:"+chalID).Result()
		if err != nil {
			t.Fatalf("Redis challenge should exist before enrollment: %v", err)
		}
		if strings.Contains(val, rawSecret) {
			t.Errorf("raw enrollment token IS contained in serialized challenge value")
		}
		var chalData map[string]interface{}
		json.Unmarshal([]byte(val), &chalData)
		if _, ok := chalData["token_hash"]; ok {
			t.Errorf("token_hash IS stored in challenge value")
		}

		req := agentenrollment.AgentEnrollRequest{
			EnrollmentToken:  rawSecret,
			ChallengeID:      chalID,
			ClientInstanceID: ci,
			PublicKey:        pub,
			Signature:        signNonce(nonce, priv),
			DeviceInfo:       validDeviceInfo,
		}
		service.EnrollAgent(ctx, req)

		_, err = rdb.Get(ctx, "agent:enroll:challenge:"+chalID).Result()
		if err == nil || err != redis.Nil {
			t.Errorf("challenge key was not removed by GETDEL")
		}

		var tokenHashDB string
		pool.QueryRow(ctx, "SELECT token_hash FROM agent_enrollment_keys WHERE key_id = $1", keyID).Scan(&tokenHashDB)
		if tokenHashDB == rawSecret {
			t.Errorf("raw secret found in DB")
		}
	})

	t.Run("explicit Agent revoked", func(t *testing.T) {
		setupKey(nil, nil, nil)
		pub, priv := genKey()
		ci := "ci_agent_revoked"
		chalID, nonce, _ := service.GenerateChallenge(ctx, rawSecret, ci, pub)
		req := agentenrollment.AgentEnrollRequest{
			EnrollmentToken:  rawSecret,
			ChallengeID:      chalID,
			ClientInstanceID: ci,
			PublicKey:        pub,
			Signature:        signNonce(nonce, priv),
			DeviceInfo:       validDeviceInfo,
		}
		res, _, _ := service.EnrollAgent(ctx, req)

		// Revoke the agent explicitly
		pool.Exec(ctx, "UPDATE device_agents SET status = 'revoked', revoked_at = NOW() WHERE agent_id = $1", res.AgentID)

		// Retry with new challenge -> should fail identity conflict or unauthorized
		chalID2, nonce2, _ := service.GenerateChallenge(ctx, rawSecret, ci, pub)
		req.ChallengeID = chalID2
		req.Signature = signNonce(nonce2, priv)
		_, _, err := service.EnrollAgent(ctx, req)
		if err != agentenrollment.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized on revoked retry, got %v", err)
		}
		
		// Check that it was not reactivated
		var status string
		pool.QueryRow(ctx, "SELECT status FROM device_agents WHERE agent_id = $1", res.AgentID).Scan(&status)
		if status != "revoked" {
			t.Errorf("expected agent to remain revoked, got %s", status)
		}
	})

	t.Run("non-P256 public key", func(t *testing.T) {
		setupKey(nil, nil, nil)
		privP384, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		pubBytes, _ := x509.MarshalPKIXPublicKey(&privP384.PublicKey)
		pub := base64.StdEncoding.EncodeToString(pubBytes)
		_, _, err := service.GenerateChallenge(ctx, rawSecret, "ci_p384", pub)
		if err != agentenrollment.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized for non-P256 key, got %v", err)
		}
	})

	t.Run("malformed ASN.1 DER signature", func(t *testing.T) {
		setupKey(nil, nil, nil)
		pub, _ := genKey()
		chalID, _, _ := service.GenerateChallenge(ctx, rawSecret, "ci_malformed_sig", pub)
		req := agentenrollment.AgentEnrollRequest{
			EnrollmentToken:  rawSecret,
			ChallengeID:      chalID,
			ClientInstanceID: "ci_malformed_sig",
			PublicKey:        pub,
			Signature:        base64.StdEncoding.EncodeToString([]byte{0x30, 0x03, 0x02, 0x01, 0x01}),
			DeviceInfo:       validDeviceInfo,
		}
		_, _, err := service.EnrollAgent(ctx, req)
		if err != agentenrollment.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized for malformed signature, got %v", err)
		}
	})

	t.Run("expired challenge", func(t *testing.T) {
		setupKey(nil, nil, nil)
		pub, priv := genKey()
		ci := "ci_expired_chal"
		chalID, nonce, _ := service.GenerateChallenge(ctx, rawSecret, ci, pub)
		
		// forcibly expire in redis
		rdb.Del(ctx, "agent:enroll:challenge:"+chalID)

		req := agentenrollment.AgentEnrollRequest{
			EnrollmentToken:  rawSecret,
			ChallengeID:      chalID,
			ClientInstanceID: ci,
			PublicKey:        pub,
			Signature:        signNonce(nonce, priv),
			DeviceInfo:       validDeviceInfo,
		}
		_, _, err := service.EnrollAgent(ctx, req)
		if err != agentenrollment.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized for expired challenge, got %v", err)
		}
	})

	t.Run("challenge/client_instance_id mismatch", func(t *testing.T) {
		setupKey(nil, nil, nil)
		pub, priv := genKey()
		ci := "ci_mismatch_ci"
		chalID, nonce, _ := service.GenerateChallenge(ctx, rawSecret, ci, pub)
		req := agentenrollment.AgentEnrollRequest{
			EnrollmentToken:  rawSecret,
			ChallengeID:      chalID,
			ClientInstanceID: "ci_other",
			PublicKey:        pub,
			Signature:        signNonce(nonce, priv),
			DeviceInfo:       validDeviceInfo,
		}
		_, _, err := service.EnrollAgent(ctx, req)
		if err != agentenrollment.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("challenge/public_key mismatch", func(t *testing.T) {
		setupKey(nil, nil, nil)
		pub, priv := genKey()
		pub2, _ := genKey()
		ci := "ci_mismatch_pk"
		chalID, nonce, _ := service.GenerateChallenge(ctx, rawSecret, ci, pub)
		req := agentenrollment.AgentEnrollRequest{
			EnrollmentToken:  rawSecret,
			ChallengeID:      chalID,
			ClientInstanceID: ci,
			PublicKey:        pub2,
			Signature:        signNonce(nonce, priv),
			DeviceInfo:       validDeviceInfo,
		}
		_, _, err := service.EnrollAgent(ctx, req)
		if err != agentenrollment.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("challenge/enrollment_token mismatch", func(t *testing.T) {
		setupKey(nil, nil, nil)
		pub, priv := genKey()
		ci := "ci_mismatch_token"
		chalID, nonce, _ := service.GenerateChallenge(ctx, rawSecret, ci, pub)
		req := agentenrollment.AgentEnrollRequest{
			EnrollmentToken:  "token_other",
			ChallengeID:      chalID,
			ClientInstanceID: ci,
			PublicKey:        pub,
			Signature:        signNonce(nonce, priv),
			DeviceInfo:       validDeviceInfo,
		}
		_, _, err := service.EnrollAgent(ctx, req)
		if err != agentenrollment.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized, got %v", err)
		}
	})

	t.Run("Token-Key revoke AFTER one successful binding", func(t *testing.T) {
		setupKey(nil, nil, nil)
		pub, priv := genKey()
		ci := "ci_revoke_after_bind"
		chalID, nonce, _ := service.GenerateChallenge(ctx, rawSecret, ci, pub)
		req := agentenrollment.AgentEnrollRequest{
			EnrollmentToken:  rawSecret,
			ChallengeID:      chalID,
			ClientInstanceID: ci,
			PublicKey:        pub,
			Signature:        signNonce(nonce, priv),
			DeviceInfo:       validDeviceInfo,
		}
		res, _, err := service.EnrollAgent(ctx, req)
		if err != nil {
			t.Fatalf("initial enroll failed: %v", err)
		}

		// Revoke the key
		pool.Exec(ctx, "UPDATE agent_enrollment_keys SET revoked_at = NOW() WHERE key_id = $1", keyID)

		// Assert existing agent and binding remain unchanged and active
		var aStatus string
		var bReleasedAt *time.Time
		pool.QueryRow(ctx, "SELECT status FROM device_agents WHERE agent_id = $1", res.AgentID).Scan(&aStatus)
		pool.QueryRow(ctx, "SELECT released_at FROM agent_key_bindings WHERE agent_id = $1 AND key_id = $2", res.AgentID, keyID).Scan(&bReleasedAt)
		if aStatus != "active" || bReleasedAt != nil {
			t.Errorf("expected active agent/binding, got agent=%s binding_released=%v", aStatus, bReleasedAt)
		}

		// Fresh NEW identity enrollment should be rejected
		pub2, _ := genKey()
		_, _, err = service.GenerateChallenge(ctx, rawSecret, "ci_fresh_reject", pub2)
		if err != agentenrollment.ErrUnauthorized {
			t.Errorf("expected ErrUnauthorized on revoked key fresh challenge, got %v", err)
		}
	})

	t.Run("device offline", func(t *testing.T) {
		setupKey(nil, nil, nil)
		pub, priv := genKey()
		ci := "ci_offline_device"
		chalID, nonce, _ := service.GenerateChallenge(ctx, rawSecret, ci, pub)
		req := agentenrollment.AgentEnrollRequest{
			EnrollmentToken:  rawSecret,
			ChallengeID:      chalID,
			ClientInstanceID: ci,
			PublicKey:        pub,
			Signature:        signNonce(nonce, priv),
			DeviceInfo:       validDeviceInfo,
		}
		res, _, _ := service.EnrollAgent(ctx, req)

		// Mark device offline
		pool.Exec(ctx, "UPDATE device_agents SET status = 'offline' WHERE agent_id = $1", res.AgentID)

		// Active binding remains
		var bReleasedAt *time.Time
		pool.QueryRow(ctx, "SELECT released_at FROM agent_key_bindings WHERE agent_id = $1 AND key_id = $2", res.AgentID, keyID).Scan(&bReleasedAt)
		if bReleasedAt != nil {
			t.Errorf("expected active binding (released_at is null), got %v", bReleasedAt)
		}

		// Quota remains occupied
		var currentBindings int
		pool.QueryRow(ctx, "SELECT COUNT(*) FROM agent_key_bindings WHERE key_id = $1 AND released_at IS NULL", keyID).Scan(&currentBindings)
		if currentBindings != 1 {
			t.Errorf("expected 1 active binding, got %d", currentBindings)
		}
	})
}
