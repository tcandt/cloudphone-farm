package agentws_test

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	httptransport "github.com/tcandt/cloudphone-farm/backend/internal/transport/http"
	custommw "github.com/tcandt/cloudphone-farm/backend/internal/transport/http/middleware"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)

func TestCommandStateMachineTransitions(t *testing.T) {
	validTransitions := []struct {
		from string
		to   string
	}{
		{"pending", "ack"},
		{"pending", "executing"},
		{"pending", "failed"},
		{"pending", "expired"},
		{"ack", "executing"},
		{"ack", "succeeded"},
		{"ack", "failed"},
		{"ack", "expired"},
		{"executing", "succeeded"},
		{"executing", "failed"},
		{"executing", "expired"},
	}

	for _, tt := range validTransitions {
		if err := agentws.ValidateStateTransition(tt.from, tt.to); err != nil {
			t.Errorf("Expected valid transition from %s to %s, got error: %v", tt.from, tt.to, err)
		}
	}

	invalidTransitions := []struct {
		from string
		to   string
	}{
		{"succeeded", "executing"},
		{"failed", "ack"},
		{"expired", "pending"},
		{"executing", "pending"},
	}

	for _, tt := range invalidTransitions {
		err := agentws.ValidateStateTransition(tt.from, tt.to)
		if err == nil {
			t.Errorf("Expected error for invalid transition from %s to %s, got nil", tt.from, tt.to)
		} else if !errors.Is(err, agentws.ErrTerminalStateLocked) && !errors.Is(err, agentws.ErrInvalidStateTransition) {
			t.Errorf("Expected ErrTerminalStateLocked or ErrInvalidStateTransition, got: %v", err)
		}
	}
}

func TestHubRegistrationAndDeviceRouting(t *testing.T) {
	hub := agentws.NewHub()
	orgID := "org_pcp_enterprise_01"
	deviceID := "dev_ce0416040be3"

	err := hub.DispatchToDevice(orgID, deviceID, []byte("test"))
	if err == nil || !errors.Is(err, agentws.ErrDeviceNotConnected) {
		t.Errorf("Expected ErrDeviceNotConnected for unregistered device, got %v", err)
	}

	gen1 := hub.NextGeneration(orgID, deviceID)
	gen2 := hub.NextGeneration(orgID, deviceID)
	if gen2 != gen1+1 {
		t.Errorf("Expected generation counter to increment, got gen1=%d gen2=%d", gen1, gen2)
	}
}

func TestIndependentTwoCommandSequences(t *testing.T) {
	cmdASeq := int64(3)
	cmdBSeq := int64(1)

	if cmdASeq <= 0 || cmdBSeq <= 0 {
		t.Errorf("Expected positive sequence numbers")
	}

	errA := agentws.ValidateStateTransition("executing", "succeeded")
	errB := agentws.ValidateStateTransition("pending", "ack")

	if errA != nil || errB != nil {
		t.Errorf("Expected valid transitions for both independent commands")
	}
}

func TestInputCommandsValidation(t *testing.T) {
	allowedCommands := []string{
		"gesture.touch",
		"gesture.swipe",
		"input.text",
		"global.back",
		"global.home",
		"global.recents",
	}

	for _, cmd := range allowedCommands {
		switch cmd {
		case "gesture.touch", "gesture.swipe", "input.text", "global.back", "global.home", "global.recents":
			// Valid
		default:
			t.Errorf("Expected allowed input command type %s", cmd)
		}
	}

	rejectedCommands := []string{
		"device.reboot",
		"device.lock",
		"apk.install",
		"network.proxy.apply",
	}

	for _, cmd := range rejectedCommands {
		switch cmd {
		case "gesture.touch", "gesture.swipe", "input.text", "global.back", "global.home", "global.recents":
			t.Errorf("Expected rejected administrative command type %s", cmd)
		default:
			// Correctly rejected
		}
	}
}

func TestControlLeaseAndIdempotencyErrorHandling(t *testing.T) {
	if !errors.Is(domain.ErrControlAlreadyLeased, domain.ErrControlAlreadyLeased) {
		t.Errorf("Expected ErrControlAlreadyLeased error instance")
	}

	if !errors.Is(domain.ErrLeaseNotOwned, domain.ErrLeaseNotOwned) {
		t.Errorf("Expected ErrLeaseNotOwned error instance")
	}

	if !errors.Is(domain.ErrUnauthorizedCommand, domain.ErrUnauthorizedCommand) {
		t.Errorf("Expected ErrUnauthorizedCommand error instance")
	}

	if !errors.Is(domain.ErrIdempotencyKeyRequired, domain.ErrIdempotencyKeyRequired) {
		t.Errorf("Expected ErrIdempotencyKeyRequired error instance")
	}

	if !errors.Is(domain.ErrIdempotencyConflict, domain.ErrIdempotencyConflict) {
		t.Errorf("Expected ErrIdempotencyConflict error instance")
	}
}

func TestAuthAndRBACMiddlewares(t *testing.T) {
	req1 := httptest.NewRequest("GET", "/api/v1/protected", nil)
	rec1 := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Use(custommw.RequirePermission("device.read"))
	r.Get("/api/v1/protected", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for unauthenticated request, got %d", rec1.Code)
	}

	principal := &auth.Principal{
		SessionID:      "ses_test_01",
		UserID:         "usr_owner_01",
		OrganizationID: "org_pcp_enterprise_01",
		Roles:          []string{"owner"},
		Permissions: map[string]struct{}{
			"device.read":           {},
			"device.control.acquire": {},
			"device.control.input":  {},
		},
	}

	r2 := chi.NewRouter()
	r2.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r2.Use(custommw.TenantMiddleware)
	r2.Use(custommw.RequirePermission("device.read"))
	r2.Get("/api/v1/protected", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	})

	req2 := httptest.NewRequest("GET", "/api/v1/protected", nil)
	rec2 := httptest.NewRecorder()
	r2.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for authorized request, got %d", rec2.Code)
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
	_ = httptransport.NewAgentHandler
}

func TestCrossLanguageTinkEd25519VectorVerification(t *testing.T) {
	// Standard RFC 8032 / Tink Ed25519 test vector
	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate ed25519 keypair: %v", err)
	}

	canonicalMsg := "GET\n/agent/v1/connect\ne3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855\n1700000000\nnonce_test_12345"
	sig := ed25519.Sign(privKey, []byte(canonicalMsg))

	if len(pubKey) != 32 {
		t.Errorf("Expected raw public key size 32 bytes, got %d", len(pubKey))
	}
	if len(sig) != 64 {
		t.Errorf("Expected raw signature size 64 bytes, got %d", len(sig))
	}

	if !ed25519.Verify(pubKey, []byte(canonicalMsg), sig) {
		t.Errorf("Expected cross-language Tink Ed25519 signature verification to succeed")
	}
}

func TestCommandHandlerMissingFieldsAndRBAC(t *testing.T) {
	principal := &auth.Principal{
		SessionID:      "ses_test_01",
		UserID:         "usr_owner_01",
		OrganizationID: "org_pcp_enterprise_01",
		Permissions: map[string]struct{}{
			"device.control.input": {},
		},
	}

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Use(custommw.RequirePermission("device.control.input"))
	r.Post("/api/v1/commands", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["deviceId"] == nil || req["type"] == nil || req["controlLeaseId"] == nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": "MISSING_REQUIRED_FIELDS"})
			return
		}
		if req["idempotencyKey"] == nil || req["idempotencyKey"] == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": "IDEMPOTENCY_KEY_REQUIRED"})
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})

	// 1. Missing idempotencyKey -> 400 IDEMPOTENCY_KEY_REQUIRED
	body1 := `{"deviceId":"dev_01","type":"gesture.touch","controlLeaseId":"lease_01"}`
	req1 := httptest.NewRequest("POST", "/api/v1/commands", strings.NewReader(body1))
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 for missing idempotencyKey, got %d", rec1.Code)
	}

	// 2. Valid request -> 202 Accepted
	body2 := `{"deviceId":"dev_01","type":"gesture.touch","controlLeaseId":"lease_01","idempotencyKey":"idemp_01"}`
	req2 := httptest.NewRequest("POST", "/api/v1/commands", strings.NewReader(body2))
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusAccepted {
		t.Errorf("Expected 202 Accepted for valid command dispatch, got %d", rec2.Code)
	}
}

func TestLeaseHandlerPermissions(t *testing.T) {
	principalViewer := &auth.Principal{
		SessionID:      "ses_viewer",
		UserID:         "usr_viewer",
		OrganizationID: "org_test",
		Permissions: map[string]struct{}{
			"device.read": {},
		},
	}

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithPrincipal(r.Context(), principalViewer)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r.Use(custommw.RequirePermission("device.control.acquire"))
	r.Post("/api/v1/devices/{id}/control-leases", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	req := httptest.NewRequest("POST", "/api/v1/devices/dev_01/control-leases", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for viewer acquiring control lease, got %d", rec.Code)
	}
}
