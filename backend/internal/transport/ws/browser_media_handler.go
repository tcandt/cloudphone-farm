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
	"github.com/tcandt/cloudphone-farm/backend/internal/cluster"
	devservice "github.com/tcandt/cloudphone-farm/backend/internal/device"
	redispkg "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
)

type BrowserMediaHandler struct {
	hub              *agentws.Hub
	deviceService    *devservice.DeviceService
	viewerRepo       *redispkg.ViewerRepository
	agentConnRepo    *redispkg.AgentConnectionRepository
	mediaSessionRepo *redispkg.MediaSessionRepository
	router           *cluster.ClusterRouter
	nodeID           string
	coturnSecret     string
	coturnHost       string
	upgrader         websocket.Upgrader
}

func NewBrowserMediaHandler(hub *agentws.Hub, deviceService *devservice.DeviceService, allowedOrigins []string, viewerRepo *redispkg.ViewerRepository) *BrowserMediaHandler {
	var coturnSecret string
	secretPath := os.Getenv("COTURN_SHARED_SECRET_FILE")
	if secretPath != "" {
		if bytes, err := os.ReadFile(secretPath); err == nil && len(bytes) > 0 {
			coturnSecret = strings.TrimSpace(string(bytes))
		}
	}
	if coturnSecret == "" {
		coturnSecret = os.Getenv("COTURN_SHARED_SECRET")
	}

	coturnHost := os.Getenv("COTURN_HOST")
	if coturnHost == "" {
		coturnHost = "turn.phonecontrol.io"
	}

	return &BrowserMediaHandler{
		hub:           hub,
		deviceService: deviceService,
		viewerRepo:    viewerRepo,
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

func (h *BrowserMediaHandler) SetClusterComponents(
	nodeID string,
	agentConnRepo *redispkg.AgentConnectionRepository,
	mediaSessionRepo *redispkg.MediaSessionRepository,
	router *cluster.ClusterRouter,
) {
	h.nodeID = nodeID
	h.agentConnRepo = agentConnRepo
	h.mediaSessionRepo = mediaSessionRepo
	h.router = router
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

	// 3. Distributed Viewer Quota Lease Check (MAX_DIRECT_P2P_VIEWERS_PER_DEVICE = 1)
	sessionID := fmt.Sprintf("sess_%s", uuid.New().String()[:8])
	if h.viewerRepo != nil {
		if err := h.viewerRepo.AcquireViewerLease(ctx, orgID, deviceID, sessionID, 15*time.Minute); err != nil {
			slog.Warn("Viewer quota exceeded for device stream", "device_id", deviceID, "org_id", orgID, "error", err)
			http.Error(w, "Maximum viewer stream limit reached for device (max 1)", http.StatusTooManyRequests)
			return
		}
		defer func() {
			_ = h.viewerRepo.ReleaseViewerLease(context.Background(), orgID, deviceID, sessionID)
		}()
	}

	// 4. Verify Active Agent Connection Owner (Distributed Directory or Local Hub)
	var ownerNodeID string
	var snap agentws.ConnectionSnapshot

	if h.agentConnRepo != nil {
		ownerRec, err := h.agentConnRepo.GetOwner(ctx, orgID, deviceID)
		if err != nil || ownerRec == nil {
			slog.Warn("Device agent owner directory lookup failed or not connected", "device_id", deviceID, "org_id", orgID, "error", err)
			http.Error(w, "Device Agent Not Connected", http.StatusServiceUnavailable)
			return
		}
		ownerNodeID = ownerRec.NodeID
		snap = agentws.ConnectionSnapshot{
			AgentID:        ownerRec.AgentID,
			ConnectionID:   ownerRec.ConnectionID,
			Generation:     ownerRec.Generation,
			OrganizationID: orgID,
			DeviceID:       deviceID,
		}
	} else {
		agentConn, ok := h.hub.GetConnection(orgID, deviceID)
		if !ok || agentConn == nil {
			slog.Warn("Device agent is not connected for browser media stream", "device_id", deviceID, "org_id", orgID)
			http.Error(w, "Device Agent Not Connected", http.StatusServiceUnavailable)
			return
		}
		ownerNodeID = h.nodeID
		snap = agentws.ConnectionSnapshot{
			AgentID:        agentConn.AgentID,
			ConnectionID:   agentConn.ConnectionID,
			Generation:     agentConn.Generation,
			OrganizationID: orgID,
			DeviceID:       deviceID,
		}
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("Failed to upgrade Browser Media WebSocket", "error", err)
		return
	}
	defer conn.Close()

	browserChan := make(chan []byte, 128)
	expiresAt := time.Now().Add(15 * time.Minute)

	// Register local browser session channel for session-scoped media routing
	agentws.DefaultMediaBrowserRegistry().Register(sessionID, browserChan)
	defer agentws.DefaultMediaBrowserRegistry().Unregister(sessionID)

	// 5. Real Coturn REST HMAC-SHA1 Ephemeral Credential Generation
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

	// 6. Register authenticated MediaSession record in local Hub and distributed Redis repository
	mediaSession := &agentws.MediaSession{
		SessionID:      sessionID,
		OrganizationID: orgID,
		DeviceID:       deviceID,
		UserID:         principal.UserID,
		AgentID:        snap.AgentID,
		ConnectionID:   snap.ConnectionID,
		Generation:     snap.Generation,
		ExpiresAt:      expiresAt,
		Subscriber:     browserChan,
	}
	h.hub.RegisterMediaSession(mediaSession)
	defer h.hub.UnregisterMediaSession(sessionID)

	if h.mediaSessionRepo != nil {
		distSession := &redispkg.DistributedMediaSession{
			SessionID:      sessionID,
			BrowserNodeID:  h.nodeID,
			AgentNodeID:    ownerNodeID,
			AgentID:        snap.AgentID,
			ConnectionID:   snap.ConnectionID,
			Generation:     snap.Generation,
			OrganizationID: orgID,
			DeviceID:       deviceID,
			UserID:         principal.UserID,
			CreatedAt:      time.Now().UTC(),
			ExpiresAt:      expiresAt.UTC(),
		}
		_ = h.mediaSessionRepo.RegisterMediaSession(ctx, distSession, 15*time.Minute)
		defer func() {
			_ = h.mediaSessionRepo.UnregisterMediaSession(context.Background(), sessionID)
		}()
	}

	slog.Info("Browser WebRTC Media Signaling session created", "session_id", sessionID, "device_id", deviceID, "org_id", orgID, "user_id", principal.UserID, "agent_node_id", ownerNodeID)

	// 7. Send media.session.created server-owned frame to Browser first
	createdPayload := map[string]interface{}{
		"session_id":  sessionID,
		"device_id":   deviceID,
		"org_id":      orgID,
		"user_id":     principal.UserID,
		"expires_at":  expiresAt.UTC().Format(time.RFC3339Nano),
		"ice_servers": iceServers,
	}
	createdEnv, _ := agentws.NewWSEnvelope(agentws.WSMessageType("media.session.created"), "msg_created_01", createdPayload)
	createdBytes, _ := json.Marshal(createdEnv)
	_ = conn.WriteMessage(websocket.TextMessage, createdBytes)

	// 8. Dispatch media.session.start to Device Agent Node via ClusterRouter or local Hub
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

	var dispatchErr error
	if h.router != nil {
		dispatchErr = h.router.SendMediaSignalToAgent(ctx, sessionID, ownerNodeID, snap, startBytes)
	} else {
		dispatchErr = h.hub.DispatchToMediaSession(sessionID, startBytes)
	}

	if dispatchErr != nil {
		slog.Warn("Device agent not connected or failed to receive media.session.start", "device_id", deviceID, "error", dispatchErr)
		errPayload := map[string]interface{}{"session_id": sessionID, "status": "failed", "error_message": dispatchErr.Error()}
		errEnv, _ := agentws.NewWSEnvelope(agentws.MessageTypeError, "err_01", errPayload)
		errBytes, _ := json.Marshal(errEnv)
		_ = conn.WriteMessage(websocket.TextMessage, errBytes)
		return
	}

	var wg sync.WaitGroup
	ctxCancel, cancel := context.WithCancel(ctx)
	defer cancel()

	// Expiry Timer: Immediate stop & close when TTL expires
	expiryTimer := time.NewTimer(time.Until(expiresAt))
	defer expiryTimer.Stop()

	go func() {
		select {
		case <-ctxCancel.Done():
			return
		case <-expiryTimer.C:
			slog.Info("Media session reached TTL expiration. Forcefully closing session.", "session_id", sessionID)
			stopPayload := map[string]interface{}{"session_id": sessionID, "reason": "session_ttl_expired"}
			stopEnv, _ := agentws.NewWSEnvelope(agentws.MessageTypeMediaSessionStop, "msg_ttl_stop", stopPayload)
			stopBytes, _ := json.Marshal(stopEnv)

			if h.router != nil {
				_ = h.router.SendMediaSignalToAgent(context.Background(), sessionID, ownerNodeID, snap, stopBytes)
			} else {
				_ = h.hub.DispatchStopToMediaSession(sessionID, stopBytes)
			}

			_ = conn.SetReadDeadline(time.Now())
			_ = conn.Close()
			cancel()
		}
	}()

	// Goroutine 1: Relay messages from Browser -> Device Agent Node
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

			if time.Now().After(expiresAt) {
				slog.Warn("Browser signaling message received post-expiry. Dropping.", "session_id", sessionID)
				break
			}

			var env agentws.WSEnvelope
			if err := json.Unmarshal(message, &env); err != nil {
				continue
			}

			var payloadMap map[string]interface{}
			if err := json.Unmarshal(env.Payload, &payloadMap); err != nil {
				slog.Warn("Browser message payload unmarshal error. Dropping.", "session_id", sessionID)
				continue
			}

			reqSessID, ok := payloadMap["session_id"].(string)
			if !ok || reqSessID == "" || reqSessID != sessionID {
				slog.Warn("Browser message missing or invalid session_id. Dropping message.", "expected", sessionID, "got", reqSessID)
				continue
			}

			switch env.Type {
			case agentws.MessageTypeMediaSignalOffer, agentws.MessageTypeMediaSignalCandidate:
				if h.router != nil {
					_ = h.router.SendMediaSignalToAgent(ctx, sessionID, ownerNodeID, snap, message)
				} else {
					_ = h.hub.DispatchToMediaSession(sessionID, message)
				}
			case agentws.MessageTypeMediaSessionStop:
				if h.router != nil {
					_ = h.router.SendMediaSignalToAgent(ctx, sessionID, ownerNodeID, snap, message)
				} else {
					_ = h.hub.DispatchStopToMediaSession(sessionID, message)
				}
			}
		}
	}()

	// Goroutine 2: Relay messages from Device Agent Node (browserChan) -> Browser
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

	if h.router != nil {
		_ = h.router.SendMediaSignalToAgent(context.Background(), sessionID, ownerNodeID, snap, stopBytes)
	} else {
		_ = h.hub.DispatchStopToMediaSession(sessionID, stopBytes)
	}

	slog.Info("Browser Media Session closed cleanly", "session_id", sessionID)
}
