package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentkey"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

type AgentKeyHandler struct {
	svc agentkey.AgentKeyService
}

func NewAgentKeyHandler(svc agentkey.AgentKeyService) *AgentKeyHandler {
	return &AgentKeyHandler{svc: svc}
}

func (h *AgentKeyHandler) Create(w http.ResponseWriter, r *http.Request) {
	principal, err := auth.GetPrincipal(r.Context())
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	var req PostAgentKeysJSONBody
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body")
		return
	}

	svcReq := agentkey.CreateKeyRequest{
		Name:        req.Name,
		MaxBindings: req.MaxBindings,
	}
	if req.ExpiresAt != nil {
		svcReq.ExpiresAt = req.ExpiresAt
	}

	key, rawSecret, err := h.svc.CreateKey(r.Context(), principal.OrganizationID, principal.UserID, svcReq)
	if err != nil {
		if errors.Is(err, agentkey.ErrInvalidParams) {
			WriteError(w, http.StatusBadRequest, "INVALID_PARAMS", "Invalid parameters for key")
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to create agent key")
		return
	}

	res := AgentKeyCreatedResponse{
		Key:       mapDomainKeyToAPI(key),
		RawSecret: rawSecret,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}

func (h *AgentKeyHandler) List(w http.ResponseWriter, r *http.Request) {
	principal, err := auth.GetPrincipal(r.Context())
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	keys, err := h.svc.ListKeys(r.Context(), principal.OrganizationID)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list agent keys")
		return
	}

	var res []AgentKey
	for _, k := range keys {
		res = append(res, mapDomainKeyToAPI(k))
	}
	if res == nil {
		res = []AgentKey{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (h *AgentKeyHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	principal, err := auth.GetPrincipal(r.Context())
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	keyID := chi.URLParam(r, "keyId")

	key, err := h.svc.GetKey(r.Context(), principal.OrganizationID, keyID)
	if err != nil {
		if errors.Is(err, agentkey.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Agent key not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get agent key")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mapDomainKeyToAPI(key))
}

func (h *AgentKeyHandler) Update(w http.ResponseWriter, r *http.Request) {
	principal, err := auth.GetPrincipal(r.Context())
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	keyID := chi.URLParam(r, "keyId")

	var rawReq map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&rawReq); err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body")
		return
	}

	svcReq := agentkey.UpdateKeyRequest{}
	
	if nameRaw, ok := rawReq["name"]; ok {
		if nameRaw != nil {
			if str, isStr := nameRaw.(string); isStr {
				svcReq.Name = &str
			}
		} else {
			// explicitly null, which is invalid for name, let service return error
			empty := ""
			svcReq.Name = &empty
		}
	}
	
	if maxBRaw, ok := rawReq["max_bindings"]; ok {
		svcReq.UpdateMaxBindings = true
		if maxBRaw != nil {
			if num, isNum := maxBRaw.(float64); isNum {
				val := int(num)
				svcReq.MaxBindings = &val
			}
		}
	}
	
	if expRaw, ok := rawReq["expires_at"]; ok {
		svcReq.UpdateExpiresAt = true
		if expRaw != nil {
			if str, isStr := expRaw.(string); isStr {
				if t, err := time.Parse(time.RFC3339, str); err == nil {
					svcReq.ExpiresAt = &t
				}
			}
		}
	}

	key, err := h.svc.UpdateKey(r.Context(), principal.OrganizationID, keyID, svcReq)
	if err != nil {
		if errors.Is(err, agentkey.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Agent key not found")
			return
		}
		if errors.Is(err, agentkey.ErrInvalidParams) {
			WriteError(w, http.StatusBadRequest, "INVALID_PARAMS", "Invalid parameters for key")
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update agent key")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mapDomainKeyToAPI(key))
}

func (h *AgentKeyHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	principal, err := auth.GetPrincipal(r.Context())
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	keyID := chi.URLParam(r, "keyId")

	err = h.svc.RevokeKey(r.Context(), principal.OrganizationID, keyID)
	if err != nil {
		if errors.Is(err, agentkey.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Agent key not found or already revoked")
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to revoke agent key")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}



func mapDomainKeyToAPI(k *domain.AgentKey) AgentKey {
	ak := AgentKey{
		KeyId:          k.KeyID,
		OrganizationId: k.OrganizationID,
		CreatedBy:      k.CreatedBy,
		Name:           k.Name,
		TokenPrefix:    k.TokenPrefix,
		CreatedAt:      k.CreatedAt,
		UpdatedAt:      k.UpdatedAt,
	}
	if k.MaxBindings != nil {
		ak.MaxBindings = k.MaxBindings
	}
	if k.ExpiresAt != nil {
		ak.ExpiresAt = k.ExpiresAt
	}
	if k.RevokedAt != nil {
		ak.RevokedAt = k.RevokedAt
	}
	if k.LastUsedAt != nil {
		ak.LastUsedAt = k.LastUsedAt
	}
	return ak
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"code":      code,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
