package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	"github.com/tcandt/cloudphone-farm/backend/internal/device"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

type DeviceHandler struct {
	deviceService *device.DeviceService
}

func NewDeviceHandler(deviceService *device.DeviceService) *DeviceHandler {
	return &DeviceHandler{deviceService: deviceService}
}

func (h *DeviceHandler) List(w http.ResponseWriter, r *http.Request) {
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

	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))

	params := domain.DeviceListParams{
		Page:    page,
		Limit:   limit,
		Status:  q.Get("status"),
		GroupID: q.Get("group_id"),
		Search:  q.Get("search"),
	}

	result, err := h.deviceService.ListDevices(r.Context(), principal.OrganizationID, params)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":      "INTERNAL_SERVER_ERROR",
			"message":   "Failed to list organization devices",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func (h *DeviceHandler) GetByID(w http.ResponseWriter, r *http.Request) {
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

	deviceID := chi.URLParam(r, "id")
	if deviceID == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":      "INVALID_DEVICE_ID",
			"message":   "Device ID parameter is required",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	dev, err := h.deviceService.GetDeviceByID(r.Context(), principal.OrganizationID, deviceID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if errors.Is(err, domain.ErrDeviceNotFound) {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "DEVICE_NOT_FOUND",
				"message":   "Requested device was not found",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "INTERNAL_SERVER_ERROR",
				"message":   "Failed to retrieve device details",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dev)
}
