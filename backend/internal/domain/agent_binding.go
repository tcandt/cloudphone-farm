package domain

import "time"

type AgentKeyBinding struct {
	BindingID            string     `json:"binding_id"`
	OrganizationID       string     `json:"organization_id"`
	KeyID                string     `json:"key_id"`
	DeviceID             string     `json:"device_id"`
	AgentID              string     `json:"agent_id"`
	PublicKeyFingerprint string     `json:"public_key_fingerprint"`
	BoundAt              time.Time  `json:"bound_at"`
	ReleasedAt           *time.Time `json:"released_at,omitempty"`
	ReleaseReason        *string    `json:"release_reason,omitempty"`
}
