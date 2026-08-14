package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tcandt/cloudphone-farm/backend/internal/agent"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	pgrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
	custommw "github.com/tcandt/cloudphone-farm/backend/internal/transport/http/middleware"
)

type AgentHandler struct {
	agentService *agent.AgentService
}

func NewAgentHandler(agentService *agent.AgentService) *AgentHandler {
	return &AgentHandler{agentService: agentService}
}

func (h *AgentHandler) CreateToken(w http.ResponseWriter, r *http.Request) {
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

	var req agent.CreateTokenRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	issued, err := h.agentService.CreateEnrollmentToken(r.Context(), principal, req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":      "INTERNAL_SERVER_ERROR",
			"message":   "Failed to issue enrollment token",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(issued)
}

func (h *AgentHandler) ListTokens(w http.ResponseWriter, r *http.Request) {
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

	tokens, err := h.agentService.ListEnrollmentTokens(r.Context(), principal.OrganizationID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":      "INTERNAL_SERVER_ERROR",
			"message":   "Failed to list enrollment tokens",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(tokens)
}

func (h *AgentHandler) RevokeToken(w http.ResponseWriter, r *http.Request) {
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

	tokenID := chi.URLParam(r, "id")
	if err := h.agentService.RevokeEnrollmentToken(r.Context(), principal.OrganizationID, tokenID); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":      "TOKEN_NOT_FOUND",
			"message":   "Enrollment token not found or already consumed",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *AgentHandler) EnrollAgent(w http.ResponseWriter, r *http.Request) {
	var req agent.EnrollRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":      "INVALID_JSON",
			"message":   "Malformed JSON request body",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	res, err := h.agentService.EnrollAgent(r.Context(), req)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if errors.Is(err, pgrepo.ErrTokenInvalidOrConsumed) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "TOKEN_INVALID_OR_CONSUMED",
				"message":   "Enrollment token is invalid, expired, revoked, or already consumed",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "INTERNAL_SERVER_ERROR",
				"message":   "Agent enrollment failed",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}

func (h *AgentHandler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	agentObj := r.Context().Value(custommw.AgentContextKey)
	if agentObj == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":      "AGENT_UNAUTHENTICATED",
			"message":   "Agent cryptographic identity missing from context",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	deviceAgent, ok := agentObj.(*domain.DeviceAgent)
	if !ok || deviceAgent == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":      "AGENT_UNAUTHENTICATED",
			"message":   "Invalid agent context payload",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	var req agent.HeartbeatRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":      "INVALID_JSON",
			"message":   "Malformed JSON request body",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	if err := h.agentService.ProcessHeartbeat(r.Context(), deviceAgent, req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":      "INTERNAL_SERVER_ERROR",
			"message":   "Failed to process heartbeat presence",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ack"})
}
