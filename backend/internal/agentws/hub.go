package agentws

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
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

type Hub struct {
	connections   map[string]*Connection   // Keyed by deviceKey (org_id:device_id)
	generations   map[string]int64        // Generation tracking per deviceKey
	mediaSessions map[string]*MediaSession // Keyed by session_id
	mu            sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		connections:   make(map[string]*Connection),
		generations:   make(map[string]int64),
		mediaSessions: make(map[string]*MediaSession),
	}
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
	existing, exists := h.connections[key]
	if exists && existing != nil {
		slog.Info("Closing existing WebSocket connection for device reconnect", "device_key", key, "old_gen", existing.Generation, "new_gen", conn.Generation)
		go existing.Close()
	}

	h.connections[key] = conn
	slog.Info("Registered active agent WebSocket connection", "device_key", key, "agent_id", conn.AgentID, "connection_id", conn.ConnectionID, "generation", conn.Generation)
}

func (h *Hub) Unregister(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := DeviceKey(conn.OrganizationID, conn.DeviceID)
	existing, exists := h.connections[key]
	if exists && existing != nil && existing.ConnectionID == conn.ConnectionID && existing.Generation == conn.Generation {
		delete(h.connections, key)
		slog.Info("Unregistered agent WebSocket connection", "device_key", key, "connection_id", conn.ConnectionID)
	}
}

func (h *Hub) GetConnection(orgID, deviceID string) (*Connection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	key := DeviceKey(orgID, deviceID)
	conn, exists := h.connections[key]
	return conn, exists
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
	h.mu.RUnlock()

	if !exists || session == nil {
		return ErrSessionNotFound
	}

	// Strict identity and connection fencing verification: Agent connection must match session registry snapshot
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
