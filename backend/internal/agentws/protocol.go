package agentws

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidStateTransition = errors.New("invalid command state transition")
	ErrStaleSequence          = errors.New("stale status sequence ignored")
	ErrTerminalStateLocked    = errors.New("command is in terminal state and cannot be modified")
)

type WSMessageType string

const (
	MessageTypeServerChallenge        WSMessageType = "server.challenge"
	MessageTypeAgentChallengeResponse WSMessageType = "agent.challenge_response"
	MessageTypeConnectionReady        WSMessageType = "connection.ready"
	MessageTypeHeartbeat              WSMessageType = "heartbeat"
	MessageTypeHeartbeatACK           WSMessageType = "heartbeat.ack"
	MessageTypeCommandDispatch        WSMessageType = "command.dispatch"
	MessageTypeCommandStatus          WSMessageType = "command.status"
	MessageTypePing                   WSMessageType = "ping"
	MessageTypePong                   WSMessageType = "pong"
	MessageTypeError                  WSMessageType = "error"
	MessageTypeMediaSessionStart      WSMessageType = "media.session.start"
	MessageTypeMediaSessionReady      WSMessageType = "media.session.ready"
	MessageTypeMediaSessionStarted    WSMessageType = "media.session.started"
	MessageTypeMediaSessionStop       WSMessageType = "media.session.stop"
	MessageTypeMediaSessionStopped    WSMessageType = "media.session.stopped"
	MessageTypeMediaSignalOffer       WSMessageType = "media.signal.offer"
	MessageTypeMediaSignalAnswer      WSMessageType = "media.signal.answer"
	MessageTypeMediaSignalCandidate   WSMessageType = "media.signal.candidate"
)

type WSEnvelope struct {
	Version   int             `json:"version"`
	Type      WSMessageType   `json:"type"`
	MessageID string          `json:"message_id"`
	Timestamp string          `json:"timestamp"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type ServerChallengePayload struct {
	ChallengeNonce string `json:"challenge_nonce"`
	ExpiresAt      string `json:"expires_at"`
}

type AgentChallengeResponsePayload struct {
	ChallengeSignature string `json:"challenge_signature"`
}

type ConnectionReadyPayload struct {
	ConnectionID string `json:"connection_id"`
	Generation   int64  `json:"generation"`
	AgentID      string `json:"agent_id"`
	DeviceID     string `json:"device_id"`
}

type CommandDispatchPayload struct {
	CommandID    string                 `json:"command_id"`
	DeviceID     string                 `json:"device_id"`
	CommandType  string                 `json:"command_type"`
	Payload      map[string]interface{} `json:"payload"`
	ControlLease string                 `json:"control_lease_id,omitempty"`
	FencingToken int64                  `json:"fencing_token,omitempty"`
	IssuedAt     string                 `json:"issued_at"`
	ExpiresAt    string                 `json:"expires_at"`
}

type CommandStatusPayload struct {
	CommandID    string `json:"command_id"`
	Status       string `json:"status"` // ack, executing, succeeded, failed, expired
	Sequence     int64  `json:"sequence"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// IsTerminalState checks if a command status is a final state
func IsTerminalState(status string) bool {
	switch status {
	case "succeeded", "failed", "expired":
		return true
	default:
		return false
	}
}

// ValidateStateTransition checks strict command state machine rules
func ValidateStateTransition(currentStatus, newStatus string) error {
	if IsTerminalState(currentStatus) {
		return fmt.Errorf("%w: current state is %s", ErrTerminalStateLocked, currentStatus)
	}

	if currentStatus == newStatus {
		return nil // Idempotent same-state ACK
	}

	switch currentStatus {
	case "pending":
		if newStatus != "ack" && newStatus != "executing" && newStatus != "failed" && newStatus != "expired" {
			return fmt.Errorf("%w: cannot transition from pending to %s", ErrInvalidStateTransition, newStatus)
		}
	case "ack":
		if newStatus != "executing" && newStatus != "succeeded" && newStatus != "failed" && newStatus != "expired" {
			return fmt.Errorf("%w: cannot transition from ack to %s", ErrInvalidStateTransition, newStatus)
		}
	case "executing":
		if newStatus != "succeeded" && newStatus != "failed" && newStatus != "expired" {
			return fmt.Errorf("%w: cannot transition from executing to %s", ErrInvalidStateTransition, newStatus)
		}
	default:
		return fmt.Errorf("%w: unknown state %s", ErrInvalidStateTransition, currentStatus)
	}

	return nil
}

func NewWSEnvelope(msgType WSMessageType, messageID string, payload interface{}) (*WSEnvelope, error) {
	var payloadBytes []byte
	var err error
	if payload != nil {
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal WS payload: %w", err)
		}
	}

	return &WSEnvelope{
		Version:   1,
		Type:      msgType,
		MessageID: messageID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Payload:   payloadBytes,
	}, nil
}
