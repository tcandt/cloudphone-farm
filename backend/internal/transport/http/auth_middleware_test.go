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

func TestAuthAndRBACMiddlewares(t *testing.T) {
	// 1. Unauthenticated Request -> 401
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

	// 2. Authenticated Principal with matching permission -> 200 OK
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

	// 3. Authenticated Principal missing required permission -> 403 Forbidden
	r3 := chi.NewRouter()
	r3.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r3.Use(custommw.TenantMiddleware)
	r3.Use(custommw.RequirePermission("organization.manage"))
	r3.Get("/api/v1/admin", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req3 := httptest.NewRequest("GET", "/api/v1/admin", nil)
	rec3 := httptest.NewRecorder()
	r3.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for missing permission, got %d", rec3.Code)
	}
}

func TestCSRFMiddleware(t *testing.T) {
	allowedOrigins := []string{"http://localhost:3000"}
	csrf := custommw.CSRFMiddleware(allowedOrigins)

	r := chi.NewRouter()
	r.Use(csrf)
	r.Post("/api/v1/command", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req1 := httptest.NewRequest("POST", "/api/v1/command", nil)
	req1.Header.Set("Origin", "http://malicious-attacker.com")
	rec1 := httptest.NewRecorder()
	r.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for malicious origin, got %d", rec1.Code)
	}

	req2 := httptest.NewRequest("POST", "/api/v1/command", nil)
	req2.Header.Set("Origin", "http://localhost:3000")
	rec2 := httptest.NewRecorder()
	r.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for allowed origin, got %d", rec2.Code)
	}
}
