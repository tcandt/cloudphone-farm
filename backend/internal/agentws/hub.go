package agentws

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/tcandt/cloudphone-farm/backend/internal/telemetry"
)

var (
	ErrDeviceNotConnected       = errors.New("device agent is not connected to WebSocket hub")
	ErrBufferOverflow           = errors.New("connection send channel buffer overflow")
	ErrSessionNotFound          = errors.New("media session subscriber not found")
	ErrUnauthorizedMediaSession = errors.New("unauthorized media session relay attempt")
)

type MediaSession struct {
	SessionID      string
	OrganizationID string
	DeviceID       string
	AgentID        string
	UserID         string
	ConnectionID   string
	Generation     int64
	ExpiresAt      time.Time
	Subscriber     chan []byte
}

type ConnectionSnapshot struct {
	AgentID        string `json:"agent_id"`
	ConnectionID   string `json:"connection_id"`
	Generation     int64  `json:"generation"`
	OrganizationID string `json:"organization_id,omitempty"`
	DeviceID       string `json:"device_id,omitempty"`
}

type DistributedMediaRelayer interface {
	RelayMediaSignalToBrowser(ctx context.Context, conn *Connection, sessionID string, data []byte) error
}

type Hub struct {
	connections   map[string]*Connection   // Keyed by deviceKey (org_id:device_id)
	generations   map[string]int64        // Generation tracking per deviceKey
	mediaSessions map[string]*MediaSession // Keyed by session_id
	relayer       DistributedMediaRelayer
	mu            sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		connections:   make(map[string]*Connection),
		generations:   make(map[string]int64),
		mediaSessions: make(map[string]*MediaSession),
	}
}

func (h *Hub) SetDistributedMediaRelayer(relayer DistributedMediaRelayer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.relayer = relayer
}

func DeviceKey(orgID, deviceID string) string {
	return fmt.Sprintf("%s:%s", orgID, deviceID)
}

func (h *Hub) NextGeneration(orgID, deviceID string) int64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := DeviceKey(orgID, deviceID)
	h.generations[key]++
	return h.generations[key]
}

func (h *Hub) Register(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := DeviceKey(conn.OrganizationID, conn.DeviceID)

	// Single Active Connection Enforcement: Disconnect old connection if generation is lower or stale
	if oldConn, exists := h.connections[key]; exists && oldConn != nil {
		slog.Info("Disconnecting stale WebSocket connection for device", "device_key", key, "old_connection_id", oldConn.ConnectionID, "old_gen", oldConn.Generation, "new_connection_id", conn.ConnectionID, "new_gen", conn.Generation)
		go oldConn.Close()
	}

	h.connections[key] = conn
	telemetry.GetMetrics().IncrAgentConnections("connected")
	slog.Info("Registered device agent WebSocket connection in hub", "device_key", key, "connection_id", conn.ConnectionID, "generation", conn.Generation)
}

func (h *Hub) Unregister(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := DeviceKey(conn.OrganizationID, conn.DeviceID)
	if existing, exists := h.connections[key]; exists && existing == conn {
		delete(h.connections, key)
		telemetry.GetMetrics().IncrAgentConnections("disconnected")
		slog.Info("Unregistered device agent WebSocket connection from hub", "device_key", key, "connection_id", conn.ConnectionID, "generation", conn.Generation)
	}
}

// CloseConnectionForAgent closes matching active WebSocket connection for an agent immediately upon revocation
func (h *Hub) CloseConnectionForAgent(orgID, deviceID, agentID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := DeviceKey(orgID, deviceID)
	conn, exists := h.connections[key]
	if exists && conn != nil && (agentID == "" || conn.AgentID == agentID) {
		slog.Warn("Closing active Agent WebSocket connection due to revocation event", "device_key", key, "agent_id", conn.AgentID, "conn_id", conn.ConnectionID)
		go conn.Close()
		delete(h.connections, key)
		return true
	}
	return false
}

func (h *Hub) GetConnection(orgID, deviceID string) (*Connection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	key := DeviceKey(orgID, deviceID)
	conn, exists := h.connections[key]
	return conn, exists && conn != nil
}

func (h *Hub) GetConnectionSnapshot(orgID, deviceID string) (ConnectionSnapshot, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	key := DeviceKey(orgID, deviceID)
	conn, exists := h.connections[key]
	if !exists || conn == nil {
		return ConnectionSnapshot{}, false
	}

	return ConnectionSnapshot{
		AgentID:        conn.AgentID,
		ConnectionID:   conn.ConnectionID,
		Generation:     conn.Generation,
		OrganizationID: conn.OrganizationID,
		DeviceID:       conn.DeviceID,
	}, true
}

func (h *Hub) DispatchToConnectionSnapshot(orgID, deviceID string, snap ConnectionSnapshot, data []byte) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	key := DeviceKey(orgID, deviceID)
	conn, exists := h.connections[key]
	if !exists || conn == nil {
		return ErrDeviceNotConnected
	}

	if conn.AgentID != snap.AgentID || conn.ConnectionID != snap.ConnectionID || conn.Generation != snap.Generation {
		slog.Warn("Stale connection generation fencing mismatch for command dispatch. Dropping.",
			"device_key", key,
			"snap_conn_id", snap.ConnectionID, "curr_conn_id", conn.ConnectionID,
			"snap_gen", snap.Generation, "curr_gen", conn.Generation)
		return ErrUnauthorizedMediaSession
	}

	select {
	case conn.Send <- data:
		return nil
	default:
		slog.Error("Disconnecting slow agent consumer due to command send buffer overflow", "device_key", key, "connection_id", conn.ConnectionID)
		go conn.Close()
		return fmt.Errorf("%w: device %s buffer full", ErrBufferOverflow, key)
	}
}

func (h *Hub) DispatchToDevice(orgID, deviceID string, data []byte) error {
	h.mu.RLock()
	key := DeviceKey(orgID, deviceID)
	conn, exists := h.connections[key]
	h.mu.RUnlock()

	if !exists || conn == nil {
		return ErrDeviceNotConnected
	}

	select {
	case conn.Send <- data:
		return nil
	default:
		slog.Error("Disconnecting slow agent consumer due to send buffer overflow", "device_key", key, "connection_id", conn.ConnectionID)
		go conn.Close()
		return fmt.Errorf("%w: device %s buffer full", ErrBufferOverflow, key)
	}
}

func (h *Hub) DispatchToMediaSession(sessionID string, data []byte) error {
	h.mu.RLock()
	session, sessionExists := h.mediaSessions[sessionID]
	if !sessionExists || session == nil {
		h.mu.RUnlock()
		return ErrSessionNotFound
	}

	if time.Now().After(session.ExpiresAt) {
		h.mu.RUnlock()
		return errors.New("media session expired")
	}

	key := DeviceKey(session.OrganizationID, session.DeviceID)
	conn, connExists := h.connections[key]
	h.mu.RUnlock()

	if !connExists || conn == nil {
		return ErrDeviceNotConnected
	}

	// Connection Fencing: Verify Agent identity, connection ID, and generation match media session snapshot
	if conn.AgentID != session.AgentID || conn.ConnectionID != session.ConnectionID || conn.Generation != session.Generation {
		slog.Warn("Stale connection generation fencing mismatch for media session dispatch. Dropping.",
			"session_id", sessionID,
			"sess_conn_id", session.ConnectionID, "curr_conn_id", conn.ConnectionID,
			"sess_gen", session.Generation, "curr_gen", conn.Generation)
		return ErrUnauthorizedMediaSession
	}

	select {
	case conn.Send <- data:
		return nil
	default:
		slog.Error("Media dispatch failed due to agent send channel buffer full", "session_id", sessionID)
		return ErrBufferOverflow
	}
}

func (h *Hub) DispatchStopToMediaSession(sessionID string, data []byte) error {
	h.mu.RLock()
	session, sessionExists := h.mediaSessions[sessionID]
	if !sessionExists || session == nil {
		h.mu.RUnlock()
		return ErrSessionNotFound
	}

	key := DeviceKey(session.OrganizationID, session.DeviceID)
	conn, connExists := h.connections[key]
	h.mu.RUnlock()

	if !connExists || conn == nil {
		return ErrDeviceNotConnected
	}

	// Connection Fencing: Verify Agent identity, connection ID, and generation match media session snapshot
	if conn.AgentID != session.AgentID || conn.ConnectionID != session.ConnectionID || conn.Generation != session.Generation {
		slog.Warn("Stale connection generation fencing mismatch for media session stop dispatch. Dropping.",
			"session_id", sessionID,
			"sess_conn_id", session.ConnectionID, "curr_conn_id", conn.ConnectionID,
			"sess_gen", session.Generation, "curr_gen", conn.Generation)
		return ErrUnauthorizedMediaSession
	}

	select {
	case conn.Send <- data:
		return nil
	default:
		slog.Error("Media stop dispatch failed due to agent send channel buffer full", "session_id", sessionID)
		return ErrBufferOverflow
	}
}

func (h *Hub) RegisterMediaSession(session *MediaSession) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.mediaSessions[session.SessionID] = session
	slog.Info("Registered authenticated WebRTC MediaSession record", "session_id", session.SessionID, "org_id", session.OrganizationID, "device_id", session.DeviceID, "user_id", session.UserID, "agent_id", session.AgentID, "conn_id", session.ConnectionID, "gen", session.Generation)
}

func (h *Hub) UnregisterMediaSession(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.mediaSessions, sessionID)
	slog.Info("Unregistered WebRTC MediaSession record", "session_id", sessionID)
}

func (h *Hub) RelayMediaSignalFromAgent(conn *Connection, sessionID string, data []byte) error {
	h.mu.RLock()
	session, exists := h.mediaSessions[sessionID]
	relayer := h.relayer
	h.mu.RUnlock()

	if exists && session != nil {
		// Strict identity and connection fencing verification
		if conn.OrganizationID != session.OrganizationID ||
			conn.DeviceID != session.DeviceID ||
			conn.AgentID != session.AgentID ||
			conn.ConnectionID != session.ConnectionID ||
			conn.Generation != session.Generation {
			slog.Warn("Agent identity / connection generation fencing mismatch for MediaSession relay attempt. Rejecting.",
				"session_id", sessionID,
				"agent_org", conn.OrganizationID, "sess_org", session.OrganizationID,
				"agent_dev", conn.DeviceID, "sess_dev", session.DeviceID,
				"agent_id", conn.AgentID, "sess_agent_id", session.AgentID,
				"agent_conn_id", conn.ConnectionID, "sess_conn_id", session.ConnectionID,
				"agent_gen", conn.Generation, "sess_gen", session.Generation)
			return ErrUnauthorizedMediaSession
		}

		if time.Now().After(session.ExpiresAt) {
			slog.Warn("MediaSession expired. Rejecting relay.", "session_id", sessionID)
			return errors.New("media session expired")
		}

		select {
		case session.Subscriber <- data:
			return nil
		default:
			slog.Warn("MediaSession subscriber channel full, dropping frame", "session_id", sessionID)
			return ErrBufferOverflow
		}
	}

	if relayer != nil {
		return relayer.RelayMediaSignalToBrowser(context.Background(), conn, sessionID, data)
	}

	return ErrSessionNotFound
}
