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

func (h *AgentKeyHandler) RegisterRoutes(r chi.Router) {
	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/{keyId}", h.GetByID)
	r.Patch("/{keyId}", h.Update)
	r.Delete("/{keyId}", h.Revoke)
	r.Get("/{keyId}/devices", h.GetBindings)
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

	var rawReq map[string]json.RawMessage
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rawReq); err != nil {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Invalid request body")
		return
	}
	if decoder.More() {
		WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "Trailing JSON data not allowed")
		return
	}

	svcReq := agentkey.UpdateKeyRequest{}
	
	for k, v := range rawReq {
		switch k {
		case "name":
			if string(v) == "null" {
				WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "name cannot be null")
				return
			}
			var name string
			if err := json.Unmarshal(v, &name); err != nil {
				WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid name format")
				return
			}
			svcReq.Name = &name
		case "max_bindings":
			svcReq.UpdateMaxBindings = true
			if string(v) != "null" {
				var max int
				if err := json.Unmarshal(v, &max); err != nil {
					WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid max_bindings format")
					return
				}
				svcReq.MaxBindings = &max
			}
		case "expires_at":
			svcReq.UpdateExpiresAt = true
			if string(v) != "null" {
				var expStr string
				if err := json.Unmarshal(v, &expStr); err != nil {
					WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid expires_at format")
					return
				}
				exp, err := time.Parse(time.RFC3339, expStr)
				if err != nil {
					WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "expires_at must be valid RFC3339 timestamp")
					return
				}
				svcReq.ExpiresAt = &exp
			}
		default:
			WriteError(w, http.StatusBadRequest, "BAD_REQUEST", "unrecognized or immutable field: "+k)
			return
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

func (h *AgentKeyHandler) GetBindings(w http.ResponseWriter, r *http.Request) {
	principal, err := auth.GetPrincipal(r.Context())
	if err != nil {
		WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	keyID := chi.URLParam(r, "keyId")

	bindings, err := h.svc.GetBindings(r.Context(), principal.OrganizationID, keyID)
	if err != nil {
		if errors.Is(err, agentkey.ErrNotFound) {
			WriteError(w, http.StatusNotFound, "NOT_FOUND", "Agent key not found")
			return
		}
		WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get bindings")
		return
	}

	var res []AgentKeyBinding
	for _, b := range bindings {
		res = append(res, mapDomainBindingToAPI(b))
	}
	if res == nil {
		res = []AgentKeyBinding{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
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
	ak.ActiveBindings = k.ActiveBindings
	return ak
}

func mapDomainBindingToAPI(b *domain.AgentKeyBinding) AgentKeyBinding {
	ab := AgentKeyBinding{
		BindingId:            b.BindingID,
		DeviceId:             b.DeviceID,
		AgentId:              b.AgentID,
		PublicKeyFingerprint: b.PublicKeyFingerprint,
		BoundAt:              b.BoundAt,
	}
	if b.ReleasedAt != nil {
		ab.ReleasedAt = b.ReleasedAt
	}
	if b.ReleaseReason != nil {
		ab.ReleaseReason = b.ReleaseReason
	}
	return ab
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
