package agentws

import (
	"log/slog"
	"sync"
)

type LocalMediaSubscriber struct {
	SessionID string
	Send      chan []byte
}

type MediaBrowserRegistry struct {
	sessions map[string]*LocalMediaSubscriber
	mu       sync.RWMutex
}

var defaultMediaBrowserRegistry = NewMediaBrowserRegistry()

func DefaultMediaBrowserRegistry() *MediaBrowserRegistry {
	return defaultMediaBrowserRegistry
}

func NewMediaBrowserRegistry() *MediaBrowserRegistry {
	return &MediaBrowserRegistry{
		sessions: make(map[string]*LocalMediaSubscriber),
	}
}

func (r *MediaBrowserRegistry) Register(sessionID string, sendChan chan []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[sessionID] = &LocalMediaSubscriber{
		SessionID: sessionID,
		Send:      sendChan,
	}
	slog.Info("Registered local media browser subscriber for session", "session_id", sessionID)
}

func (r *MediaBrowserRegistry) Unregister(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.sessions, sessionID)
	slog.Info("Unregistered local media browser subscriber for session", "session_id", sessionID)
}

func (r *MediaBrowserRegistry) SendToSession(sessionID string, data []byte) bool {
	r.mu.RLock()
	sub, exists := r.sessions[sessionID]
	r.mu.RUnlock()

	if !exists || sub == nil {
		return false
	}

	select {
	case sub.Send <- data:
		return true
	default:
		slog.Warn("Local media browser subscriber channel full, dropping frame", "session_id", sessionID)
		return false
	}
}
