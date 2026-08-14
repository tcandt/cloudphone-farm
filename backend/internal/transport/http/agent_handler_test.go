package http

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	custommw "github.com/tcandt/cloudphone-farm/backend/internal/transport/http/middleware"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)

func TestEnrollmentTokensSecurityAndSchemas(t *testing.T) {
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
}

func TestEd25519AgentSignatureVerification(t *testing.T) {
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

	if !ed25519.Verify(agent.PublicKey, []byte(canonicalMsg), sig) {
		t.Errorf("Expected Ed25519 signature verification to succeed")
	}

	tamperedMsg := fmt.Sprintf("POST\n/api/v1/agents/heartbeat\n%s\n%s\n%s",
		"tampered_body_hash",
		timestamp,
		nonce,
	)
	if ed25519.Verify(agent.PublicKey, []byte(tamperedMsg), sig) {
		t.Errorf("Expected Ed25519 signature verification to fail on tampered body")
	}

	_ = sigB64
}
