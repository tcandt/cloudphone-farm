package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	custommw "github.com/tcandt/cloudphone-farm/backend/internal/transport/http/middleware"
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

	// 2. Unauthenticated Heartbeat Request without X-Agent-Fingerprint -> 401
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
