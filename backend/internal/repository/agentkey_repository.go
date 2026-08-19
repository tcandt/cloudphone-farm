package repository

import (
	"context"
	"time"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

type AgentKeyRepository interface {
	Create(ctx context.Context, key *domain.AgentKey) error
	GetByID(ctx context.Context, orgID, keyID string) (*domain.AgentKey, error)
	List(ctx context.Context, orgID string) ([]*domain.AgentKey, error)
	Update(ctx context.Context, orgID, keyID string, name string, maxBindings *int, expiresAt *time.Time) (*domain.AgentKey, error)
	Revoke(ctx context.Context, orgID, keyID string) error
}
