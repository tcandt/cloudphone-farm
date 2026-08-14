package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	"github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
)

type AuthMiddleware struct {
	authService *auth.AuthService
	cookieName  string
}

func NewAuthMiddleware(authService *auth.AuthService, cookieName string) *AuthMiddleware {
	if cookieName == "" {
		cookieName = "__Host-pcp_session"
	}
	return &AuthMiddleware{
		authService: authService,
		cookieName:  cookieName,
	}
}

func (m *AuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(m.cookieName)
		if err != nil || cookie.Value == "" {
			// Fallback check dev cookie name if main cookie missing
			if m.cookieName != "pcp_session_dev" {
				if devCookie, devErr := r.Cookie("pcp_session_dev"); devErr == nil && devCookie.Value != "" {
					cookie = devCookie
				}
			}
		}

		if cookie == nil || cookie.Value == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "UNAUTHENTICATED",
				"message":   "Authentication session cookie missing or empty",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		principal, err := m.authService.GetSessionByToken(r.Context(), cookie.Value)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			if errors.Is(err, redis.ErrRedisDown) {
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code":      "SERVICE_UNAVAILABLE",
					"message":   "Authoritative session store temporarily unavailable",
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				})
			} else {
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"code":      "UNAUTHENTICATED",
					"message":   "Session token invalid, expired, or revoked",
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				})
			}
			return
		}

		// Inject principal into request context
		ctx := auth.WithPrincipal(r.Context(), principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
