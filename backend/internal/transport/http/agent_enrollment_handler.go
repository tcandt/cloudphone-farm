package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentenrollment"
)

type AgentEnrollmentHandlerV2 struct {
	service *agentenrollment.EnrollmentV2Service
}

func NewAgentEnrollmentHandlerV2(service *agentenrollment.EnrollmentV2Service) *AgentEnrollmentHandlerV2 {
	return &AgentEnrollmentHandlerV2{
		service: service,
	}
}

func (h *AgentEnrollmentHandlerV2) RegisterRoutes(r chi.Router) {
	// These routes are mounted outside of auth middleware
	r.Post("/challenge", h.RequestChallenge)
	r.Post("/", h.FinalizeEnrollment)
}

func (h *AgentEnrollmentHandlerV2) RequestChallenge(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EnrollmentToken  string `json:"enrollment_token"`
		ClientInstanceID string `json:"client_instance_id"`
		PublicKey        string `json:"public_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	if req.EnrollmentToken == "" || req.ClientInstanceID == "" || req.PublicKey == "" {
		h.writeError(w, http.StatusBadRequest, "BAD_REQUEST", "missing required fields")
		return
	}

	challengeID, nonce, err := h.service.GenerateChallenge(r.Context(), req.EnrollmentToken, req.ClientInstanceID, req.PublicKey)
	if err != nil {
		if errors.Is(err, agentenrollment.ErrUnauthorized) {
			h.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid enrollment credentials")
			return
		}
		h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"challenge_id": challengeID,
		"nonce":        nonce,
	})
}

func (h *AgentEnrollmentHandlerV2) FinalizeEnrollment(w http.ResponseWriter, r *http.Request) {
	var req agentenrollment.AgentEnrollRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	if req.EnrollmentToken == "" || req.ChallengeID == "" || req.ClientInstanceID == "" || req.PublicKey == "" || req.Signature == "" {
		h.writeError(w, http.StatusBadRequest, "BAD_REQUEST", "missing required fields")
		return
	}

	res, created, err := h.service.EnrollAgent(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, agentenrollment.ErrUnauthorized):
			h.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid enrollment credentials or challenge")
		case errors.Is(err, agentenrollment.ErrIdentityConflict):
			h.writeError(w, http.StatusConflict, "IDENTITY_CONFLICT", "identity conflict detected")
		case errors.Is(err, agentenrollment.ErrQuotaExhausted):
			h.writeError(w, http.StatusConflict, "ENROLLMENT_QUOTA_EXHAUSTED", "enrollment quota exhausted")
		case errors.Is(err, agentenrollment.ErrInvalidRequest):
			h.writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request")
		default:
			h.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if created {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(res)
}

func (h *AgentEnrollmentHandlerV2) writeError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":      code,
		"message":   msg,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
