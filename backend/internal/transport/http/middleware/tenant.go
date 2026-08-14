package middleware

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
)

// TenantMiddleware enforces organization boundary exclusively from the authenticated session Principal
func TenantMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, err := auth.GetPrincipal(r.Context())
		if err != nil || principal.OrganizationID == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "TENANT_ISOLATION_FAILURE",
				"message":   "No active organization tenant associated with session",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}
