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
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
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

	r := chi.NewRouter()
	r.Route("/api/v2/agents/enroll", func(r chi.Router) {
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

	// Add more complex tests if needed, but since we proved the domain in integration test,
	// just asserting the routing handlers respond correctly to typical inputs covers the contract.
}
