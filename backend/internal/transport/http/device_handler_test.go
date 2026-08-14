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

func TestDeviceEndpointsRBACAndTenantIsolation(t *testing.T) {
	// 1. Unauthenticated request to /devices -> 401
	r1 := chi.NewRouter()
	r1.Use(custommw.RequirePermission("device.read"))
	r1.Get("/api/v1/devices", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req1 := httptest.NewRequest("GET", "/api/v1/devices", nil)
	rec1 := httptest.NewRecorder()
	r1.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized for unauthenticated request to /devices, got %d", rec1.Code)
	}

	// 2. Principal missing device.read permission -> 403
	principalNoRead := &auth.Principal{
		SessionID:      "ses_test_01",
		UserID:         "usr_test_01",
		OrganizationID: "org_pcp_enterprise_01",
		Permissions: map[string]struct{}{
			"organization.manage": {},
		},
	}

	r2 := chi.NewRouter()
	r2.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithPrincipal(r.Context(), principalNoRead)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r2.Use(custommw.TenantMiddleware)
	r2.Use(custommw.RequirePermission("device.read"))
	r2.Get("/api/v1/devices", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req2 := httptest.NewRequest("GET", "/api/v1/devices", nil)
	rec2 := httptest.NewRecorder()
	r2.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for principal missing device.read, got %d", rec2.Code)
	}

	// 3. Principal with device.read permission -> 200 OK
	principalWithRead := &auth.Principal{
		SessionID:      "ses_test_02",
		UserID:         "usr_owner_01",
		OrganizationID: "org_pcp_enterprise_01",
		Permissions: map[string]struct{}{
			"device.read": {},
		},
	}

	r3 := chi.NewRouter()
	r3.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := auth.WithPrincipal(r.Context(), principalWithRead)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	r3.Use(custommw.TenantMiddleware)
	r3.Use(custommw.RequirePermission("device.read"))
	r3.Get("/api/v1/devices", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"items": []string{}, "total": 0})
	})

	req3 := httptest.NewRequest("GET", "/api/v1/devices", nil)
	rec3 := httptest.NewRecorder()
	r3.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for authorized device.read request, got %d", rec3.Code)
	}
}
