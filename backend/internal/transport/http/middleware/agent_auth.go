package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	pgrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
)

type agentCtxKey string

const AgentContextKey agentCtxKey = "authenticated_agent"

type AgentAuthMiddleware struct {
	enrollRepo *pgrepo.EnrollmentRepository
}

func NewAgentAuthMiddleware(enrollRepo *pgrepo.EnrollmentRepository) *AgentAuthMiddleware {
	return &AgentAuthMiddleware{enrollRepo: enrollRepo}
}

func (m *AgentAuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fingerprint := r.Header.Get("X-Agent-Fingerprint")
		if fingerprint == "" {
			fingerprint = r.Header.Get("X-Agent-ID")
		}

		if fingerprint == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "AGENT_UNAUTHENTICATED",
				"message":   "Agent cryptographic fingerprint header missing",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		agent, err := m.enrollRepo.GetAgentByFingerprint(r.Context(), fingerprint)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "AGENT_CREDENTIAL_INVALID",
				"message":   "Agent credential invalid or revoked",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		ctx := context.WithValue(r.Context(), AgentContextKey, agent)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
