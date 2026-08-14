package agentws

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

type Connection struct {
	ConnectionID   string
	OrganizationID string
	DeviceID       string
	AgentID        string
	Generation     int64
	Agent          *domain.DeviceAgent

	ws           *websocket.Conn
	hub          *Hub
	Send         chan []byte
	once         sync.Once
	closed       chan struct{}
	lastSequence int64
	seqMutex     sync.Mutex
}

func NewConnection(hub *Hub, ws *websocket.Conn, agent *domain.DeviceAgent, generation int64) *Connection {
	connID := fmt.Sprintf("conn_%s", uuid.New().String()[:12])
	return &Connection{
		ConnectionID:   connID,
		OrganizationID: agent.OrganizationID,
		DeviceID:       agent.DeviceID,
		AgentID:        agent.AgentID,
		Generation:     generation,
		Agent:          agent,
		ws:             ws,
		hub:            hub,
		Send:           make(chan []byte, 128), // Bounded buffer 128
		closed:         make(chan struct{}),
	}
}

func (c *Connection) Close() {
	c.once.Do(func() {
		close(c.closed)
		close(c.Send)
		_ = c.ws.Close()
		c.hub.Unregister(c)
	})
}

func (c *Connection) ReadLoop(ctx context.Context, statusCallback func(payload CommandStatusPayload) error) {
	defer c.Close()

	c.ws.SetReadLimit(64 * 1024)
	_ = c.ws.SetReadDeadline(time.Now().Add(30 * time.Second))
	c.ws.SetPongHandler(func(string) error {
		_ = c.ws.SetReadDeadline(time.Now().Add(30 * time.Second))
		return nil
	})

	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Warn("WebSocket read error", "connection_id", c.ConnectionID, "agent_id", c.AgentID, "error", err)
			}
			break
		}

		var env WSEnvelope
		if err := json.Unmarshal(message, &env); err != nil {
			slog.Warn("Malformed WS message received", "error", err)
			continue
		}

		switch env.Type {
		case MessageTypePing:
			pongEnv, _ := NewWSEnvelope(MessageTypePong, env.MessageID, nil)
			pongData, _ := json.Marshal(pongEnv)
			_ = c.ws.WriteMessage(websocket.TextMessage, pongData)

		case MessageTypeCommandStatus:
			var statusPayload CommandStatusPayload
			if err := json.Unmarshal(env.Payload, &statusPayload); err == nil {
				c.seqMutex.Lock()
				if statusPayload.Sequence <= c.lastSequence {
					c.seqMutex.Unlock()
					slog.Warn("Ignored stale command status sequence", "seq", statusPayload.Sequence, "last_seq", c.lastSequence)
					continue
				}
				c.lastSequence = statusPayload.Sequence
				c.seqMutex.Unlock()

				if statusCallback != nil {
					_ = statusCallback(statusPayload)
				}
			}
		}
	}
}

func (c *Connection) WriteLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.ws.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			// Add queued frames
			n := len(c.Send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
			pingEnv, _ := NewWSEnvelope(MessageTypePing, fmt.Sprintf("ping_%d", time.Now().UnixNano()), nil)
			pingData, _ := json.Marshal(pingEnv)
			if err := c.ws.WriteMessage(websocket.TextMessage, pingData); err != nil {
				return
			}

		case <-c.closed:
			return
		}
	}
}

func VerifyChallengeResponse(challengeNonce string, signatureB64 string, pubKey []byte) error {
	sigBytes, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		sigBytes, err = base64.URLEncoding.DecodeString(signatureB64)
	}
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		return errors.New("invalid signature format")
	}

	if !ed25519.Verify(pubKey, []byte(challengeNonce), sigBytes) {
		return errors.New("challenge signature verification failed")
	}

	return nil
}
