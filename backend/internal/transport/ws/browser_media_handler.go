package ws

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	devservice "github.com/tcandt/cloudphone-farm/backend/internal/device"
)

type BrowserMediaHandler struct {
	hub           *agentws.Hub
	deviceService *devservice.DeviceService
	coturnSecret  string
	coturnHost    string
	upgrader      websocket.Upgrader
}

func NewBrowserMediaHandler(hub *agentws.Hub, deviceService *devservice.DeviceService, allowedOrigins []string) *BrowserMediaHandler {
	coturnSecret := os.Getenv("COTURN_SHARED_SECRET")
	if coturnSecret == "" {
		coturnSecret = "pcp_coturn_secret_key"
	}
	coturnHost := os.Getenv("COTURN_HOST")
	if coturnHost == "" {
		coturnHost = "turn.phonecontrol.io"
	}

	return &BrowserMediaHandler{
		hub:           hub,
		deviceService: deviceService,
		coturnSecret:  coturnSecret,
		coturnHost:    coturnHost,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				u, err := url.Parse(origin)
				if err != nil {
					return false
				}
				if strings.EqualFold(u.Host, r.Host) {
					return true
				}
				for _, allowed := range allowedOrigins {
					if strings.EqualFold(origin, allowed) || allowed == "*" {
						return true
					}
				}
				return false
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

	// 1. Strict Tenant Authority & Authentication Check
	principal, err := auth.GetPrincipal(ctx)
	if err != nil || principal == nil {
		slog.Warn("Unauthorized browser media WebSocket connection attempt")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	deviceID := chi.URLParam(r, "id")
	if deviceID == "" {
		slog.Warn("Missing device ID in path for browser media connection")
		http.Error(w, "Missing Device ID", http.StatusBadRequest)
		return
	}

	orgID := principal.OrganizationID

	// 2. Validate Tenant Ownership: Device must belong to principal's organization
	if h.deviceService != nil {
		device, err := h.deviceService.GetDeviceByID(ctx, orgID, deviceID)
		if err != nil || device == nil {
			slog.Warn("Device tenant isolation check failed for browser media stream", "device_id", deviceID, "org_id", orgID, "error", err)
			http.Error(w, "Device Not Found or Access Denied", http.StatusNotFound)
			return
		}
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade Browser Media WebSocket", "error", err)
		return
	}
	defer conn.Close()

	sessionID := fmt.Sprintf("sess_%s", uuid.New().String()[:8])
	browserChan := make(chan []byte, 128)

	// 3. Real Coturn REST HMAC-SHA1 Ephemeral Credential Generation
	unixExpiry := time.Now().Add(10 * time.Minute).Unix()
	username := fmt.Sprintf("%d:%s", unixExpiry, sessionID)

	mac := hmac.New(sha1.New, []byte(h.coturnSecret))
	mac.Write([]byte(username))
	credential := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	iceServers := []ICEServer{
		{URLs: []string{fmt.Sprintf("stun:%s:3478", h.coturnHost)}},
		{
			URLs: []string{
				fmt.Sprintf("turn:%s:3478?transport=udp", h.coturnHost),
				fmt.Sprintf("turn:%s:3478?transport=tcp", h.coturnHost),
			},
			Username:   username,
			Credential: credential,
		},
	}

	// Register browser subscriber in agentws Hub
	h.hub.RegisterMediaSubscriber(sessionID, browserChan)
	defer h.hub.UnregisterMediaSubscriber(sessionID)

	slog.Info("Browser WebRTC Media Signaling session created", "session_id", sessionID, "device_id", deviceID, "org_id", orgID, "user_id", principal.UserID)

	// 4. Send media.session.created server-owned frame to Browser first so Browser has session_id and Coturn REST ICE config
	createdPayload := map[string]interface{}{
		"session_id":  sessionID,
		"device_id":   deviceID,
		"org_id":      orgID,
		"ice_servers": iceServers,
	}
	createdEnv, _ := agentws.NewWSEnvelope(agentws.WSMessageType("media.session.created"), "msg_created_01", createdPayload)
	createdBytes, _ := json.Marshal(createdEnv)
	_ = conn.WriteMessage(websocket.TextMessage, createdBytes)

	// 5. Dispatch media.session.start to Device Agent via Hub
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

	// Goroutine 1: Relay messages from Browser -> Device Agent (Validating session_id)
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

			var payloadMap map[string]interface{}
			if err := json.Unmarshal(env.Payload, &payloadMap); err == nil {
				if reqSessID, ok := payloadMap["session_id"].(string); ok && reqSessID != sessionID {
					slog.Warn("SessionID mismatch in incoming browser signaling message. Dropping.", "expected", sessionID, "got", reqSessID)
					continue
				}
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
