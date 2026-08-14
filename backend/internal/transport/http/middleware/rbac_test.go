package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
)

func RequirePermission(permissionCode string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, err := auth.GetPrincipal(r.Context())
			if err != nil || principal == nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if !principal.HasPermission(permissionCode) {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func TestRBACPermissionEnforcement(t *testing.T) {
	// Viewer principal template (Only stream view)
	viewerPrincipal := &auth.Principal{
		UserID:         "usr_viewer_01",
		OrganizationID: "org_test",
		Roles:          []string{"Viewer"},
		Permissions: map[string]struct{}{
			"device.read":        {},
			"device.stream.view": {},
		},
	}

	// Operator principal template (Control acquire & input allowed)
	operatorPrincipal := &auth.Principal{
		UserID:         "usr_operator_01",
		OrganizationID: "org_test",
		Roles:          []string{"Operator"},
		Permissions: map[string]struct{}{
			"device.read":            {},
			"device.stream.view":     {},
			"device.control.acquire": {},
			"device.control.input":   {},
		},
	}

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	acquireMiddleware := RequirePermission("device.control.acquire")
	inputMiddleware := RequirePermission("device.control.input")

	// 1. Viewer trying to acquire lease -> 403 Forbidden
	reqAcquire := httptest.NewRequest("POST", "/api/v1/devices/dev_01/lease", nil)
	reqAcquire = reqAcquire.WithContext(auth.WithPrincipal(context.Background(), viewerPrincipal))
	rrAcquire := httptest.NewRecorder()

	acquireMiddleware(dummyHandler).ServeHTTP(rrAcquire, reqAcquire)
	if rrAcquire.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Viewer acquiring control lease, got %d", rrAcquire.Code)
	}

	// 2. Viewer trying to dispatch command -> 403 Forbidden
	reqCmd := httptest.NewRequest("POST", "/api/v1/commands", nil)
	reqCmd = reqCmd.WithContext(auth.WithPrincipal(context.Background(), viewerPrincipal))
	rrCmd := httptest.NewRecorder()

	inputMiddleware(dummyHandler).ServeHTTP(rrCmd, reqCmd)
	if rrCmd.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Viewer dispatching command, got %d", rrCmd.Code)
	}

	// 3. Operator acquiring lease -> 200 OK
	reqOpAcquire := httptest.NewRequest("POST", "/api/v1/devices/dev_01/lease", nil)
	reqOpAcquire = reqOpAcquire.WithContext(auth.WithPrincipal(context.Background(), operatorPrincipal))
	rrOpAcquire := httptest.NewRecorder()

	acquireMiddleware(dummyHandler).ServeHTTP(rrOpAcquire, reqOpAcquire)
	if rrOpAcquire.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for Operator acquiring control lease, got %d", rrOpAcquire.Code)
	}

	// 4. Operator dispatching command -> 200 OK
	reqOpCmd := httptest.NewRequest("POST", "/api/v1/commands", nil)
	reqOpCmd = reqOpCmd.WithContext(auth.WithPrincipal(context.Background(), operatorPrincipal))
	rrOpCmd := httptest.NewRecorder()

	inputMiddleware(dummyHandler).ServeHTTP(rrOpCmd, reqOpCmd)
	if rrOpCmd.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for Operator dispatching command, got %d", rrOpCmd.Code)
	}
}
