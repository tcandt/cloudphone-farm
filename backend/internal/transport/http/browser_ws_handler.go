package http

import (
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	devservice "github.com/tcandt/cloudphone-farm/backend/internal/device"
)

type BrowserWSHandler struct {
	browserHub     *agentws.BrowserHub
	deviceService  *devservice.DeviceService
	upgrader       websocket.Upgrader
	allowedOrigins []string
}

func NewBrowserWSHandler(browserHub *agentws.BrowserHub, deviceService *devservice.DeviceService, allowedOrigins []string) *BrowserWSHandler {
	h := &BrowserWSHandler{
		browserHub:     browserHub,
		deviceService:  deviceService,
		allowedOrigins: allowedOrigins,
	}
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				// Reject missing Origin header for browser WebSocket session cookie security
				return false
			}
			for _, allowed := range allowedOrigins {
				if allowed == "*" || strings.EqualFold(origin, allowed) {
					return true
				}
			}
			return false
		},
	}
	return h
}

func (h *BrowserWSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	principal, err := auth.GetPrincipal(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	deviceID := chi.URLParam(r, "id")
	if deviceID == "" {
		writeJSONError(w, http.StatusBadRequest, "MISSING_DEVICE_ID", "Device ID is required in URL path /devices/{id}/events/ws")
		return
	}

	// Verify device exists and belongs to authenticated organization
	if h.deviceService != nil {
		dev, err := h.deviceService.GetDeviceByID(r.Context(), principal.OrganizationID, deviceID)
		if err != nil || dev == nil || dev.OrganizationID != principal.OrganizationID {
			writeJSONError(w, http.StatusNotFound, "DEVICE_NOT_FOUND", "Device not found or unauthorized")
			return
		}
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade browser event WebSocket connection", "error", err, "device_id", deviceID)
		return
	}

	subscriberID := uuid.New().String()
	subscriber := agentws.NewBrowserSubscriber(subscriberID, principal.OrganizationID, deviceID, principal.UserID)

	h.browserHub.Subscribe(subscriber)

	var closeOnce sync.Once
	cleanup := func() {
		closeOnce.Do(func() {
			h.browserHub.Unsubscribe(subscriber)
			_ = conn.Close()
		})
	}

	// Writer loop
	go func() {
		defer cleanup()

		for {
			select {
			case <-subscriber.Done:
				return
			case msg, ok := <-subscriber.Send:
				if !ok {
					return
				}
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					slog.Warn("Browser event WS write failed, closing connection", "subscriber_id", subscriberID, "error", err)
					return
				}
			}
		}
	}()

	// Reader loop (keep-alive ping/pong & graceful close)
	go func() {
		defer cleanup()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()
}
