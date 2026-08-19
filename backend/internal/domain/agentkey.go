package domain

import "time"

type AgentKey struct {
	KeyID          string     `json:"key_id"`
	OrganizationID string     `json:"organization_id"`
	CreatedBy      string     `json:"created_by"`
	Name           string     `json:"name"`
	TokenPrefix    string     `json:"token_prefix"`
	TokenHash      string     `json:"-"` // Not serialized
	MaxBindings    *int       `json:"max_bindings"`
	ExpiresAt      *time.Time `json:"expires_at"`
	RevokedAt      *time.Time `json:"revoked_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	LastUsedAt     *time.Time `json:"last_used_at"`
}
