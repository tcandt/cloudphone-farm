package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	cmdservice "github.com/tcandt/cloudphone-farm/backend/internal/command"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

type CommandHandler struct {
	cmdService *cmdservice.CommandService
}

func NewCommandHandler(cmdService *cmdservice.CommandService) *CommandHandler {
	return &CommandHandler{cmdService: cmdService}
}

func (h *CommandHandler) Dispatch(w http.ResponseWriter, r *http.Request) {
	principal, err := auth.GetPrincipal(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	var req cmdservice.DispatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_JSON_PAYLOAD", "Malformed request body")
		return
	}

	if req.DeviceID == "" || req.Type == "" || req.ControlLeaseID == "" {
		writeJSONError(w, http.StatusBadRequest, "MISSING_REQUIRED_FIELDS", "deviceId, type, and controlLeaseId are required")
		return
	}

	cmd, err := h.cmdService.DispatchCommand(r.Context(), principal.OrganizationID, principal.UserID, req)
	if err != nil {
		if errors.Is(err, domain.ErrIdempotencyKeyRequired) {
			writeJSONError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", err.Error())
			return
		}
		if errors.Is(err, domain.ErrIdempotencyConflict) {
			writeJSONError(w, http.StatusConflict, "IDEMPOTENCY_KEY_CONFLICT", err.Error())
			return
		}
		if errors.Is(err, domain.ErrUnauthorizedCommand) {
			writeJSONError(w, http.StatusForbidden, "COMMAND_NOT_PERMITTED", err.Error())
			return
		}
		if errors.Is(err, domain.ErrLeaseNotOwned) || errors.Is(err, domain.ErrLeaseNotFound) {
			writeJSONError(w, http.StatusForbidden, "CONTROL_LEASE_INVALID", "Active control lease owned by operator required")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "COMMAND_DISPATCH_REJECTED", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted) // 202 Accepted
	_ = json.NewEncoder(w).Encode(cmd)
}
