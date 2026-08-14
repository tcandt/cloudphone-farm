package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
)

type BrowserMediaHandler struct {
	hub      *agentws.Hub
	upgrader websocket.Upgrader
}

func NewBrowserMediaHandler(hub *agentws.Hub) *BrowserMediaHandler {
	return &BrowserMediaHandler{
		hub: hub,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Origin checked via CORS middleware & Auth Token
			},
		},
	}
}

type ICEServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

func (h *BrowserMediaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract device_id & org_id from query params or path
	deviceID := r.URL.Query().Get("device_id")
	orgID := r.URL.Query().Get("org_id")
	if deviceID == "" {
		deviceID = "dev_s7_edge_01"
	}
	if orgID == "" {
		orgID = "org_default"
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade Browser Media WebSocket", "error", err)
		return
	}
	defer conn.Close()

	sessionID := fmt.Sprintf("sess_%s", uuid.New().String()[:8])
	browserChan := make(chan []byte, 128)

	// Ephemeral Short-Lived TURN Credential Configuration
	iceServers := []ICEServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
		{
			URLs:       []string{"turn:stun.l.google.com:19302?transport=udp"},
			Username:   fmt.Sprintf("turn_usr_%s", sessionID),
			Credential: fmt.Sprintf("turn_pwd_%s", sessionID),
		},
	}

	// Register browser subscriber in agentws Hub
	h.hub.RegisterMediaSubscriber(sessionID, browserChan)
	defer h.hub.UnregisterMediaSubscriber(sessionID)

	slog.Info("Browser WebRTC Media Signaling session created", "session_id", sessionID, "device_id", deviceID, "org_id", orgID)

	// Dispatch media.session.start to Device Agent via Hub
	startPayload := map[string]interface{}{
		"session_id":  sessionID,
		"width":       720,
		"height":      1280,
		"bitrate":      2500000,
		"fps":         30,
		"ice_servers": iceServers,
	}
	startEnv, _ := agentws.NewWSEnvelope(agentws.MessageTypeMediaSessionStart, "msg_start_01", startPayload)
	startBytes, _ := json.Marshal(startEnv)

	if err := h.hub.DispatchToDevice(orgID, deviceID, startBytes); err != nil {
		slog.Warn("Device agent not connected or failed to receive media.session.start", "device_id", deviceID, "error", err)
		errPayload := map[string]interface{}{"session_id": sessionID, "status": "failed", "error_message": err.Error()}
		errEnv, _ := agentws.NewWSEnvelope(agentws.MessageTypeError, "err_01", errPayload)
		errBytes, _ := json.Marshal(errEnv)
		_ = conn.WriteMessage(websocket.TextMessage, errBytes)
		return
	}

	var wg sync.WaitGroup
	ctxCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	// Goroutine 1: Relay messages from Browser -> Device Agent
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()

		conn.SetReadLimit(64 * 1024)
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})

		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				break
			}

			var env agentws.WSEnvelope
			if err := json.Unmarshal(message, &env); err != nil {
				continue
			}

			switch env.Type {
			case agentws.MessageTypeMediaSignalOffer, agentws.MessageTypeMediaSignalCandidate, agentws.MessageTypeMediaSessionStop:
				// Dispatch Browser Offer/Candidate directly to Device Agent Connection
				if err := h.hub.DispatchToDevice(orgID, deviceID, message); err != nil {
					slog.Warn("Failed to dispatch browser media signal to device agent", "type", env.Type, "error", err)
				} else {
					slog.Info("Dispatched browser media signal to device agent", "type", env.Type, "session_id", sessionID)
				}
			}
		}
	}()

	// Goroutine 2: Relay messages from Device Agent (browserChan) -> Browser
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()

		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctxCancel.Done():
				return
			case message, ok := <-browserChan:
				if !ok {
					return
				}
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
					return
				}
			case <-ticker.C:
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
					return
				}
			}
		}
	}()

	wg.Wait()

	// Clean stop on disconnect
	stopPayload := map[string]interface{}{"session_id": sessionID}
	stopEnv, _ := agentws.NewWSEnvelope(agentws.MessageTypeMediaSessionStop, "msg_stop_01", stopPayload)
	stopBytes, _ := json.Marshal(stopEnv)
	_ = h.hub.DispatchToDevice(orgID, deviceID, stopBytes)
	slog.Info("Browser Media Session closed cleanly", "session_id", sessionID)
}
