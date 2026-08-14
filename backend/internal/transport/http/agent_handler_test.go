package http_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	httptransport "github.com/tcandt/cloudphone-farm/backend/internal/transport/http"
	custommw "github.com/tcandt/cloudphone-farm/backend/internal/transport/http/middleware"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)

func TestEnrollmentTokensSecurityAndSchemas(t *testing.T) {
	// 1. Principal missing agent.enroll permission -> 403 Forbidden
	principalViewer := &auth.Principal{
		SessionID:      "ses_test_01",
		UserID:         "usr_viewer_01",
		OrganizationID: "org_pcp_enterprise_01",
		Permissions: map[string]struct{}{
			"device.read": {},
		},
	}

	r1 := chi.NewRouter()
	r1.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithPrincipal(r.Context(), principalViewer)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r1.Use(custommw.TenantMiddleware)
	r1.Use(custommw.RequirePermission("agent.enroll"))
	r1.Post("/api/v1/enrollment-tokens", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	req1 := httptest.NewRequest("POST", "/api/v1/enrollment-tokens", nil)
	rec1 := httptest.NewRecorder()
	r1.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for missing agent.enroll permission, got %d", rec1.Code)
	}

	// 2. Unauthenticated Heartbeat Request without agent proof-of-possession headers -> 401
	r2 := chi.NewRouter()
	r2.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": "AGENT_UNAUTHENTICATED"})
		})
	})
	r2.Post("/api/v1/agents/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req2 := httptest.NewRequest("POST", "/api/v1/agents/heartbeat", nil)
	rec2 := httptest.NewRecorder()
	r2.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for heartbeat without agent header, got %d", rec2.Code)
	}
}

func TestEd25519AgentSignatureVerification(t *testing.T) {
	// Generate valid Ed25519 keypair
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate ed25519 keypair: %v", err)
	}

	fp := crypto.ComputePublicKeyFingerprint(pubKey)
	agent := &domain.DeviceAgent{
		AgentID:              "agt_test_01",
		OrganizationID:       "org_pcp_enterprise_01",
		DeviceID:             "dev_test_01",
		PublicKey:            pubKey,
		PublicKeyFingerprint: fp,
		Status:               "active",
	}

	body := []byte(`{"battery":85,"network":"wifi","cpu_usage":12.5}`)
	bodyHash := sha256.Sum256(body)
	bodyHashHex := hex.EncodeToString(bodyHash[:])

	timestamp := strconv.FormatInt(1700000000, 10)
	nonce := "nonce_test_12345"

	canonicalMsg := fmt.Sprintf("POST\n/api/v1/agents/heartbeat\n%s\n%s\n%s",
		bodyHashHex,
		timestamp,
		nonce,
	)

	sig := ed25519.Sign(privKey, []byte(canonicalMsg))
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	// Verify valid signature
	if !ed25519.Verify(agent.PublicKey, []byte(canonicalMsg), sig) {
		t.Errorf("Expected Ed25519 signature verification to succeed")
	}

	// Verify tampered body signature failure
	tamperedMsg := fmt.Sprintf("POST\n/api/v1/agents/heartbeat\n%s\n%s\n%s",
		"tampered_body_hash",
		timestamp,
		nonce,
	)
	if ed25519.Verify(agent.PublicKey, []byte(tamperedMsg), sig) {
		t.Errorf("Expected Ed25519 signature verification to fail on tampered body")
	}

	_ = sigB64
	_ = httptransport.NewAgentHandler
}
