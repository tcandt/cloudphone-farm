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
		
		// Ensure secret not in Redis
		val, err := rdb.Get(ctx, "chal:"+chalID).Result()
		if err == nil {
			var chalData struct {
				TokenHash string `json:"token_hash"`
			}
			json.Unmarshal([]byte(val), &chalData)
			if chalData.TokenHash == rawSecret {
				t.Errorf("raw secret found in Redis")
			}
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
		if err == nil {
			t.Errorf("expected error on revoked retry, got success")
		}
	})
}
