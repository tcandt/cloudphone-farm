package http

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
)

var browserWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Auth middleware validates cookie session & CSRF
	},
}

type BrowserWSHandler struct {
	browserHub *agentws.BrowserHub
}

func NewBrowserWSHandler(browserHub *agentws.BrowserHub) *BrowserWSHandler {
	return &BrowserWSHandler{
		browserHub: browserHub,
	}
}

func (h *BrowserWSHandler) SubscribeEvents(w http.ResponseWriter, r *http.Request) {
	principal, err := auth.GetPrincipal(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	deviceID := r.URL.Query().Get("deviceId")
	if deviceID == "" {
		// Fallback check path param if Chi path matching is used
		deviceID = r.URL.Path // Chi router context handles deviceId
	}
	if deviceID == "" {
		writeJSONError(w, http.StatusBadRequest, "MISSING_DEVICE_ID", "deviceId is required")
		return
	}

	conn, err := browserWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade browser event WebSocket connection", "error", err)
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
				slog.Warn("Browser WS write failed, closing connection", "subscriber_id", subscriberID, "error", err)
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
