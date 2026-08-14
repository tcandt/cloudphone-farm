package agentws

import (
	"errors"
	"fmt"
	"log/slog"
	"sync"
)

var (
	ErrAgentNotConnected = errors.New("agent is not connected to WebSocket hub")
	ErrBufferOverflow    = errors.New("connection send channel buffer overflow")
)

type Hub struct {
	connections map[string]*Connection // Keyed by AgentID
	generations map[string]int64      // Generation tracking per AgentID
	mu          sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[string]*Connection),
		generations: make(map[string]int64),
	}
}

func (h *Hub) NextGeneration(agentID string) int64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.generations[agentID]++
	return h.generations[agentID]
}

func (h *Hub) Register(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	existing, exists := h.connections[conn.AgentID]
	if exists && existing != nil {
		slog.Info("Closing existing connection for agent reconnect", "agent_id", conn.AgentID, "old_gen", existing.Generation, "new_gen", conn.Generation)
		go existing.Close()
	}

	h.connections[conn.AgentID] = conn
	slog.Info("Registered active agent WebSocket connection", "agent_id", conn.AgentID, "connection_id", conn.ConnectionID, "generation", conn.Generation)
}

func (h *Hub) Unregister(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	existing, exists := h.connections[conn.AgentID]
	if exists && existing != nil && existing.ConnectionID == conn.ConnectionID && existing.Generation == conn.Generation {
		delete(h.connections, conn.AgentID)
		slog.Info("Unregistered agent WebSocket connection", "agent_id", conn.AgentID, "connection_id", conn.ConnectionID)
	}
}

func (h *Hub) GetConnection(agentID string) (*Connection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	conn, exists := h.connections[agentID]
	return conn, exists
}

func (h *Hub) DispatchToAgent(agentID string, data []byte) error {
	h.mu.RLock()
	conn, exists := h.connections[agentID]
	h.mu.RUnlock()

	if !exists || conn == nil {
		return ErrAgentNotConnected
	}

	select {
	case conn.Send <- data:
		return nil
	default:
		slog.Error("Disconnecting slow agent consumer due to send buffer overflow", "agent_id", agentID, "connection_id", conn.ConnectionID)
		go conn.Close()
		return fmt.Errorf("%w: agent %s buffer full", ErrBufferOverflow, agentID)
	}
}
