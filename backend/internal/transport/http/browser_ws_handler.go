package http

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
)

type BrowserWSHandler struct {
	browserHub     *agentws.BrowserHub
	upgrader       websocket.Upgrader
	allowedOrigins []string
}

func NewBrowserWSHandler(browserHub *agentws.BrowserHub, allowedOrigins []string) *BrowserWSHandler {
	h := &BrowserWSHandler{
		browserHub:     browserHub,
		allowedOrigins: allowedOrigins,
	}
	h.upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
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
		deviceID = r.URL.Query().Get("deviceId")
	}
	if deviceID == "" {
		writeJSONError(w, http.StatusBadRequest, "MISSING_DEVICE_ID", "Device ID is required in URL path /devices/{id}/events/ws")
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade browser event WebSocket connection", "error", err, "device_id", deviceID)
		return
	}

	subscriberID := uuid.New().String()
	subscriber := &agentws.BrowserSubscriber{
		SubscriberID:   subscriberID,
		OrganizationID: principal.OrganizationID,
		DeviceID:       deviceID,
		UserID:         principal.UserID,
		Send:           make(chan []byte, 64),
	}

	h.browserHub.Subscribe(subscriber)

	// Writer loop
	go func() {
		defer func() {
			h.browserHub.Unsubscribe(subscriber)
			_ = conn.Close()
		}()

		for msg := range subscriber.Send {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				slog.Warn("Browser event WS write failed, closing connection", "subscriber_id", subscriberID, "error", err)
				return
			}
		}
	}()

	// Reader loop (keep-alive ping/pong & graceful close)
	go func() {
		defer func() {
			h.browserHub.Unsubscribe(subscriber)
			_ = conn.Close()
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()
}
