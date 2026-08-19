package agentenrollment

import (
	"context"
	"time"

	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

type EnrollmentKey struct {
	KeyID          string
	OrganizationID string
	MaxBindings    *int
	ExpiresAt      *time.Time
	RevokedAt      *time.Time
}

type IdempotencyResult struct {
	Exists      bool
	DeviceID    string
	AgentID     string
	Fingerprint string
	Status      string
	RevokedAt   *time.Time
}

type EnrollmentTx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
	
	// RevalidateKey fetches the key and locks it FOR UPDATE
	RevalidateKey(ctx context.Context, organizationID, keyID, tokenHash string) (*EnrollmentKey, error)
	
	// CheckIdempotency looks up existing binding by client_instance_id
	CheckIdempotency(ctx context.Context, organizationID, clientInstanceID string) (*IdempotencyResult, error)
	
	// CountActiveBindings counts active bindings for a key
	CountActiveBindings(ctx context.Context, organizationID, keyID string) (int, error)
	
	// CreateBinding enrolls a new agent, device, and binding
	CreateBinding(ctx context.Context, device *domain.Device, agent *domain.DeviceAgent, binding *domain.AgentKeyBinding) error
	
	// UpdateKeyLastUsedAt updates the last_used_at timestamp on the key
	UpdateKeyLastUsedAt(ctx context.Context, organizationID, keyID string) error
}

type EnrollmentV2Repository interface {
	BeginTx(ctx context.Context) (EnrollmentTx, error)
	GetTokenHashByPrefix(ctx context.Context, tokenPrefix string) (string, error)
	ResolveKey(ctx context.Context, tokenHash string) (*EnrollmentKey, error)
}
