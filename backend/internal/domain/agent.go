package domain

import "time"

type DeviceAgent struct {
	AgentID              string     `json:"agent_id"`
	OrganizationID       string     `json:"organization_id"`
	DeviceID             string     `json:"device_id"`
	ClientInstanceID     *string    `json:"client_instance_id,omitempty"`
	PublicKey            []byte     `json:"-"`
	PublicKeyFingerprint string     `json:"public_key_fingerprint"`
	ApkVersion           string     `json:"apk_version"`
	ProtocolVersion      string     `json:"protocol_version"`
	Status               string     `json:"status"`
	LastAuthenticatedAt  *time.Time `json:"last_authenticated_at,omitempty"`
}

type AgentPresencePayload struct {
	AgentID      string `json:"agent_id"`
	ConnectionID string `json:"connection_id"`
	Generation   int64  `json:"generation"`
	Sequence     int64  `json:"sequence"`
	Health       string `json:"health"`
	LastSeenAt   string `json:"last_seen_at"`
}
