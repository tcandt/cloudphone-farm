package middleware

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// CSRFMiddleware enforces Origin / Referer validation for state-changing HTTP requests
func CSRFMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowedMap := make(map[string]struct{})
	for _, o := range allowedOrigins {
		allowedMap[strings.ToLower(strings.TrimSpace(o))] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only check state-changing methods
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete {
				origin := r.Header.Get("Origin")
				if origin == "" {
					referer := r.Header.Get("Referer")
					if referer != "" {
						u, err := url.Parse(referer)
						if err == nil {
							origin = u.Scheme + "://" + u.Host
						}
					}
				}

				if origin != "" {
					normalizedOrigin := strings.ToLower(strings.TrimSpace(origin))
					if _, ok := allowedMap[normalizedOrigin]; !ok {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusForbidden)
						_ = json.NewEncoder(w).Encode(map[string]interface{}{
							"code":      "CSRF_ORIGIN_INVALID",
							"message":   "Cross-site request origin rejected",
							"timestamp": r.Context().Value("request_id"),
						})
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
