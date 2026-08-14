package middleware

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
)

// RequirePermission constructs a Chi middleware enforcing a specific permission code
func RequirePermission(code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, err := auth.GetPrincipal(r.Context())
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code":      "UNAUTHENTICATED",
					"message":   "Authentication required",
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				})
				return
			}

			if !principal.HasPermission(code) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code":      "FORBIDDEN",
					"message":   "Missing required permission: " + code,
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAnyPermission constructs a Chi middleware enforcing at least one of the provided permission codes
func RequireAnyPermission(codes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, err := auth.GetPrincipal(r.Context())
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code":      "UNAUTHENTICATED",
					"message":   "Authentication required",
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				})
				return
			}

			hasAny := false
			for _, code := range codes {
				if principal.HasPermission(code) {
					hasAny = true
					break
				}
			}

			if !hasAny {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code":      "FORBIDDEN",
					"message":   "Missing required permissions",
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
