package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	devservice "github.com/tcandt/cloudphone-farm/backend/internal/device"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

type LeaseHandler struct {
	leaseService *devservice.LeaseService
}

func NewLeaseHandler(leaseService *devservice.LeaseService) *LeaseHandler {
	return &LeaseHandler{leaseService: leaseService}
}

func (h *LeaseHandler) AcquireLease(w http.ResponseWriter, r *http.Request) {
	principal, err := auth.GetPrincipal(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	deviceID := chi.URLParam(r, "id")
	if deviceID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_DEVICE_ID", "Device ID is required")
		return
	}

	lease, err := h.leaseService.AcquireLease(r.Context(), principal.OrganizationID, deviceID, principal.UserID, principal.Email)
	if err != nil {
		if errors.Is(err, domain.ErrControlAlreadyLeased) {
			writeJSONError(w, http.StatusConflict, "CONTROL_ALREADY_LEASED", "Device control is currently leased by another operator")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "LEASE_ACQUIRE_FAILED", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(lease)
}

func (h *LeaseHandler) RenewLease(w http.ResponseWriter, r *http.Request) {
	principal, err := auth.GetPrincipal(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	deviceID := chi.URLParam(r, "id")
	leaseID := chi.URLParam(r, "leaseId")
	if deviceID == "" || leaseID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_PARAMETERS", "Device ID and Lease ID are required")
		return
	}

	lease, err := h.leaseService.RenewLease(r.Context(), principal.OrganizationID, deviceID, leaseID, principal.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrLeaseNotOwned) || errors.Is(err, domain.ErrLeaseNotFound) {
			writeJSONError(w, http.StatusForbidden, "LEASE_RENEWAL_DENIED", "Control lease not found or owned by another operator")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "LEASE_RENEW_FAILED", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(lease)
}

func (h *LeaseHandler) ReleaseLease(w http.ResponseWriter, r *http.Request) {
	principal, err := auth.GetPrincipal(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	deviceID := chi.URLParam(r, "id")
	leaseID := chi.URLParam(r, "leaseId")
	if deviceID == "" || leaseID == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_PARAMETERS", "Device ID and Lease ID are required")
		return
	}

	err = h.leaseService.ReleaseLease(r.Context(), principal.OrganizationID, deviceID, leaseID, principal.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrLeaseNotOwned) || errors.Is(err, domain.ErrLeaseNotFound) {
			writeJSONError(w, http.StatusForbidden, "LEASE_RELEASE_DENIED", "Control lease not found or owned by another operator")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "LEASE_RELEASE_FAILED", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
