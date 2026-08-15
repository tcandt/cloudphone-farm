package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	custommw "github.com/tcandt/cloudphone-farm/backend/internal/transport/http/middleware"
)

func TestProductionRBACPermissionEnforcement(t *testing.T) {
	// Viewer principal template (Only stream view & read)
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

	dummyOK := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.With(custommw.RequirePermission("device.stream.view")).Get("/stream", dummyOK)
	r.With(custommw.RequirePermission("device.control.acquire")).Post("/control-lease", dummyOK)
	r.With(custommw.RequirePermission("device.control.input")).Post("/commands", dummyOK)

	// 1. Viewer accessing stream -> 200 OK
	reqStream := httptest.NewRequest("GET", "/stream", nil)
	reqStream = reqStream.WithContext(auth.WithPrincipal(context.Background(), viewerPrincipal))
	rrStream := httptest.NewRecorder()
	r.ServeHTTP(rrStream, reqStream)
	if rrStream.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for Viewer accessing stream, got %d", rrStream.Code)
	}

	// 2. Viewer trying to acquire lease -> 403 Forbidden
	reqAcquire := httptest.NewRequest("POST", "/control-lease", nil)
	reqAcquire = reqAcquire.WithContext(auth.WithPrincipal(context.Background(), viewerPrincipal))
	rrAcquire := httptest.NewRecorder()
	r.ServeHTTP(rrAcquire, reqAcquire)
	if rrAcquire.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Viewer acquiring control lease, got %d", rrAcquire.Code)
	}

	// 3. Viewer trying to dispatch command -> 403 Forbidden
	reqCmd := httptest.NewRequest("POST", "/commands", nil)
	reqCmd = reqCmd.WithContext(auth.WithPrincipal(context.Background(), viewerPrincipal))
	rrCmd := httptest.NewRecorder()
	r.ServeHTTP(rrCmd, reqCmd)
	if rrCmd.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Viewer dispatching command, got %d", rrCmd.Code)
	}

	// 4. Operator acquiring lease -> 200 OK
	reqOpAcquire := httptest.NewRequest("POST", "/control-lease", nil)
	reqOpAcquire = reqOpAcquire.WithContext(auth.WithPrincipal(context.Background(), operatorPrincipal))
	rrOpAcquire := httptest.NewRecorder()
	r.ServeHTTP(rrOpAcquire, reqOpAcquire)
	if rrOpAcquire.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for Operator acquiring control lease, got %d", rrOpAcquire.Code)
	}

	// 5. Operator dispatching command -> 200 OK
	reqOpCmd := httptest.NewRequest("POST", "/commands", nil)
	reqOpCmd = reqOpCmd.WithContext(auth.WithPrincipal(context.Background(), operatorPrincipal))
	rrOpCmd := httptest.NewRecorder()
	r.ServeHTTP(rrOpCmd, reqOpCmd)
	if rrOpCmd.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for Operator dispatching command, got %d", rrOpCmd.Code)
	}
}
