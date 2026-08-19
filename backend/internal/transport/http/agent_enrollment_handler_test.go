package http_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentenrollment"
	"github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
	transporthttp "github.com/tcandt/cloudphone-farm/backend/internal/transport/http"
	custommw "github.com/tcandt/cloudphone-farm/backend/internal/transport/http/middleware"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
	"crypto/sha256"
	"encoding/asn1"
	"math/big"
)

func TestAgentEnrollmentHandlerV2_Integration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	redisURL := os.Getenv("REDIS_URL")
	if dbURL == "" || redisURL == "" {
		if os.Getenv("CI") == "true" {
			t.Fatal("Gate-critical HTTP test failed: CI=true but DATABASE_URL or REDIS_URL not set")
		}
		t.Skip("Skipping HTTP test; DATABASE_URL or REDIS_URL not set")
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
	handler := transporthttp.NewAgentEnrollmentHandlerV2(service)

	rateLimiter := custommw.NewRateLimiter(rdb, "test")

	r := chi.NewRouter()
	r.With(rateLimiter.LimitMiddleware(custommw.ScopeEnrollment, 50, 2)).Route("/api/v2/agents/enroll", func(r chi.Router) {
		handler.RegisterRoutes(r)
	})

	orgID := "org_v2_http"
	userID := "user_v2_http"
	pool.Exec(ctx, "INSERT INTO organizations (organization_id, name, slug) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING", orgID, "Test Org HTTP", "test-org-http")
	pool.Exec(ctx, "INSERT INTO users (user_id, email, password_hash, display_name) VALUES ($1, $2, $3, $4) ON CONFLICT DO NOTHING", userID, "http@example.com", "hash", "Test User HTTP")

	keyID := "key_http_1"
	rawSecret := "secret_http_123"
	tokenPrefix := "tc_"
	tokenHash := crypto.HashToken(rawSecret)

	setupKey := func(maxBindings *int) {
		pool.Exec(ctx, "DELETE FROM agent_key_bindings WHERE key_id = $1", keyID)
		pool.Exec(ctx, "DELETE FROM agent_enrollment_keys WHERE key_id = $1", keyID)
		_, err := pool.Exec(ctx, `
			INSERT INTO agent_enrollment_keys (key_id, organization_id, created_by, name, token_hash, token_prefix, max_bindings)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, keyID, orgID, userID, "HTTP Test Key", tokenHash, tokenPrefix, maxBindings)
		if err != nil {
			t.Fatalf("failed to insert key: %v", err)
		}
	}

	t.Run("challenge 200", func(t *testing.T) {
		setupKey(nil)
		
		priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		pubBytes, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		pubBase64 := base64.StdEncoding.EncodeToString(pubBytes)

		body := map[string]interface{}{
			"enrollment_token":   rawSecret,
			"client_instance_id": "ci_http_1",
			"public_key":         pubBase64,
		}
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/v2/agents/enroll/challenge", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d. body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("malformed request 400", func(t *testing.T) {
		req, _ := http.NewRequest("POST", "/api/v2/agents/enroll/challenge", bytes.NewReader([]byte("{malformed json")))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rr.Code)
		}
	})

	t.Run("unknown fields 400", func(t *testing.T) {
		body := map[string]interface{}{
			"enrollment_token":   rawSecret,
			"client_instance_id": "ci_http_1",
			"public_key":         "dummy",
			"unknown_field":      "bad",
		}
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/v2/agents/enroll/challenge", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for unknown fields, got %d. body: %s", rr.Code, rr.Body.String())
		}
	})

	t.Run("trailing bytes 400", func(t *testing.T) {
		body := `{"enrollment_token":"sec","client_instance_id":"ci","public_key":"pub"} {}`
		req, _ := http.NewRequest("POST", "/api/v2/agents/enroll/challenge", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected 400 for trailing bytes, got %d", rr.Code)
		}
	})

	validDeviceInfo := map[string]interface{}{
		"manufacturer":    "Test",
		"model":           "HTTP",
		"android_version": "12",
		"sdk_int":         31,
		"serial_number":   "SN-HTTP",
		"agent_version":   "1.0",
		"protocol_version": "2",
	}

	type ecdsaSignature struct {
		R, S *big.Int
	}
	signNonce := func(nonce string, priv *ecdsa.PrivateKey) string {
		nonceRaw, _ := base64.RawURLEncoding.DecodeString(nonce)
		hash := sha256.Sum256(nonceRaw)
		r, s, _ := ecdsa.Sign(rand.Reader, priv, hash[:])
		sig, _ := asn1.Marshal(ecdsaSignature{r, s})
		return base64.StdEncoding.EncodeToString(sig)
	}

	t.Run("full enrollment contract", func(t *testing.T) {
		// 1 slot
		setupKey(nil)
		pool.Exec(ctx, "UPDATE agent_enrollment_keys SET max_bindings = 1 WHERE key_id = $1", keyID)
		
		priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		pubBytes, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
		pubBase64 := base64.StdEncoding.EncodeToString(pubBytes)
		ci := "ci_contract_1"

		// challenge -> 200
		body := map[string]interface{}{
			"enrollment_token":   rawSecret,
			"client_instance_id": ci,
			"public_key":         pubBase64,
		}
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "/api/v2/agents/enroll/challenge", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. body: %s", rr.Code, rr.Body.String())
		}

		var resp struct {
			ChallengeID string `json:"challenge_id"`
			Nonce       string `json:"challenge"`
		}
		json.Unmarshal(rr.Body.Bytes(), &resp)

		// new final enrollment -> 201
		bodyEnroll := map[string]interface{}{
			"enrollment_token":   rawSecret,
			"challenge_id":       resp.ChallengeID,
			"client_instance_id": ci,
			"public_key":         pubBase64,
			"signature":          signNonce(resp.Nonce, priv),
			"device_info":        validDeviceInfo,
		}
		b2, _ := json.Marshal(bodyEnroll)
		req2, _ := http.NewRequest("POST", "/api/v2/agents/enroll", bytes.NewReader(b2))
		req2.Header.Set("Content-Type", "application/json")
		rr2 := httptest.NewRecorder()
		r.ServeHTTP(rr2, req2)
		if rr2.Code != http.StatusCreated {
			t.Errorf("expected 201 Created, got %d. body: %s", rr2.Code, rr2.Body.String())
		}

		// fresh-challenge idempotent -> 200
		req, _ = http.NewRequest("POST", "/api/v2/agents/enroll/challenge", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		rr3 := httptest.NewRecorder()
		r.ServeHTTP(rr3, req)
		if rr3.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for challenge 2, got %d", rr3.Code)
		}
		var resp3 struct {
			ChallengeID string `json:"challenge_id"`
			Nonce       string `json:"challenge"`
		}
		json.Unmarshal(rr3.Body.Bytes(), &resp3)

		bodyEnroll3 := bodyEnroll
		bodyEnroll3["challenge_id"] = resp3.ChallengeID
		bodyEnroll3["signature"] = signNonce(resp3.Nonce, priv)
		b3, _ := json.Marshal(bodyEnroll3)
		req3, _ := http.NewRequest("POST", "/api/v2/agents/enroll", bytes.NewReader(b3))
		req3.Header.Set("Content-Type", "application/json")
		rr4 := httptest.NewRecorder()
		r.ServeHTTP(rr4, req3)
		if rr4.Code != http.StatusOK {
			t.Errorf("expected 200 OK for idempotent enroll, got %d", rr4.Code)
		}

		// identity conflict -> 409
		// Use same CI but DIFFERENT key
		privOther, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		pubOtherBytes, _ := x509.MarshalPKIXPublicKey(&privOther.PublicKey)
		pubOtherBase64 := base64.StdEncoding.EncodeToString(pubOtherBytes)
		bodyConflictChallenge := map[string]interface{}{
			"enrollment_token":   rawSecret,
			"client_instance_id": ci,
			"public_key":         pubOtherBase64,
		}
		bc, _ := json.Marshal(bodyConflictChallenge)
		reqConflictChallenge, _ := http.NewRequest("POST", "/api/v2/agents/enroll/challenge", bytes.NewReader(bc))
		reqConflictChallenge.Header.Set("Content-Type", "application/json")
		rr5 := httptest.NewRecorder()
		r.ServeHTTP(rr5, reqConflictChallenge)
		if rr5.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for conflict challenge, got %d", rr5.Code)
		}
		var resp5 struct {
			ChallengeID string `json:"challenge_id"`
			Nonce       string `json:"challenge"`
		}
		json.Unmarshal(rr5.Body.Bytes(), &resp5)
		bodyConflictEnroll := map[string]interface{}{
			"enrollment_token":   rawSecret,
			"challenge_id":       resp5.ChallengeID,
			"client_instance_id": ci,
			"public_key":         pubOtherBase64,
			"signature":          signNonce(resp5.Nonce, privOther),
			"device_info":        validDeviceInfo,
		}
		bcE, _ := json.Marshal(bodyConflictEnroll)
		reqConflictEnroll, _ := http.NewRequest("POST", "/api/v2/agents/enroll", bytes.NewReader(bcE))
		reqConflictEnroll.Header.Set("Content-Type", "application/json")
		rr6 := httptest.NewRecorder()
		r.ServeHTTP(rr6, reqConflictEnroll)
		if rr6.Code != http.StatusConflict {
			t.Errorf("expected 409 Conflict for identity conflict, got %d", rr6.Code)
		}

		// quota exhausted -> 409
		// Use DIFFERENT CI, same or different key
		ciQuota := "ci_contract_quota"
		bodyQuotaChallenge := map[string]interface{}{
			"enrollment_token":   rawSecret,
			"client_instance_id": ciQuota,
			"public_key":         pubBase64,
		}
		bq, _ := json.Marshal(bodyQuotaChallenge)
		reqQuotaChallenge, _ := http.NewRequest("POST", "/api/v2/agents/enroll/challenge", bytes.NewReader(bq))
		reqQuotaChallenge.Header.Set("Content-Type", "application/json")
		rr7 := httptest.NewRecorder()
		r.ServeHTTP(rr7, reqQuotaChallenge)
		if rr7.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for quota challenge, got %d", rr7.Code)
		}
		var resp7 struct {
			ChallengeID string `json:"challenge_id"`
			Nonce       string `json:"challenge"`
		}
		json.Unmarshal(rr7.Body.Bytes(), &resp7)
		bodyQuotaEnroll := map[string]interface{}{
			"enrollment_token":   rawSecret,
			"challenge_id":       resp7.ChallengeID,
			"client_instance_id": ciQuota,
			"public_key":         pubBase64,
			"signature":          signNonce(resp7.Nonce, priv),
			"device_info":        validDeviceInfo,
		}
		bqE, _ := json.Marshal(bodyQuotaEnroll)
		reqQuotaEnroll, _ := http.NewRequest("POST", "/api/v2/agents/enroll", bytes.NewReader(bqE))
		reqQuotaEnroll.Header.Set("Content-Type", "application/json")
		rr8 := httptest.NewRecorder()
		r.ServeHTTP(rr8, reqQuotaEnroll)
		if rr8.Code != http.StatusConflict {
			t.Errorf("expected 409 Conflict for quota exhausted, got %d", rr8.Code)
		}

		// invalid credential/signature -> 401
		bodyInvalidSig := bodyEnroll
		bodyInvalidSig["signature"] = "invalid"
		bInvalidSig, _ := json.Marshal(bodyInvalidSig)
		reqInvalidSig, _ := http.NewRequest("POST", "/api/v2/agents/enroll", bytes.NewReader(bInvalidSig))
		reqInvalidSig.Header.Set("Content-Type", "application/json")
		rr9 := httptest.NewRecorder()
		r.ServeHTTP(rr9, reqInvalidSig)
		if rr9.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for invalid signature, got %d", rr9.Code)
		}

		// rate limit exhaustion -> 429
		got429 := false
		for i := 0; i < 60; i++ {
			bInvalidSig, _ := json.Marshal(bodyInvalidSig)
			reqInvalidSig, _ := http.NewRequest("POST", "/api/v2/agents/enroll", bytes.NewReader(bInvalidSig))
			reqInvalidSig.Header.Set("Content-Type", "application/json")
			reqInvalidSig.RemoteAddr = "10.0.0.99:1234"
			rrRL := httptest.NewRecorder()
			r.ServeHTTP(rrRL, reqInvalidSig)
			if rrRL.Code == http.StatusTooManyRequests {
				got429 = true
				break
			}
		}
		if !got429 {
			t.Errorf("expected at least one 429 Too Many Requests, got none")
		}
	})
}
