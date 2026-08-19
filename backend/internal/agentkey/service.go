package agentkey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	"github.com/tcandt/cloudphone-farm/backend/internal/repository"
)

var (
	ErrNotFound      = errors.New("agent key not found")
	ErrUnauthorized  = errors.New("unauthorized access to agent key")
	ErrInvalidParams = errors.New("invalid key parameters")
)

type CreateKeyRequest struct {
	Name        string
	MaxBindings *int
	ExpiresAt   *time.Time
}

type UpdateKeyRequest struct {
	Name              *string
	MaxBindings       *int
	UpdateMaxBindings bool
	ExpiresAt         *time.Time
	UpdateExpiresAt   bool
}

type AgentKeyService interface {
	CreateKey(ctx context.Context, orgID, userID string, req CreateKeyRequest) (*domain.AgentKey, string, error)
	ListKeys(ctx context.Context, orgID string) ([]*domain.AgentKey, error)
	GetKey(ctx context.Context, orgID, keyID string) (*domain.AgentKey, error)
	UpdateKey(ctx context.Context, orgID, keyID string, req UpdateKeyRequest) (*domain.AgentKey, error)
	RevokeKey(ctx context.Context, orgID, keyID string) error
}

type agentKeyService struct {
	repo repository.AgentKeyRepository
}

func NewService(repo repository.AgentKeyRepository) AgentKeyService {
	return &agentKeyService{repo: repo}
}

func (s *agentKeyService) CreateKey(ctx context.Context, orgID, userID string, req CreateKeyRequest) (*domain.AgentKey, string, error) {
	if req.MaxBindings != nil && *req.MaxBindings <= 0 {
		return nil, "", ErrInvalidParams
	}

	// Generate 32 bytes (256-bit) CSPRNG entropy
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, "", fmt.Errorf("failed to generate secure random: %w", err)
	}

	rawSecret := fmt.Sprintf("cpk_%s", hex.EncodeToString(secretBytes))

	// Hash the raw secret using SHA-256
	hash := sha256.Sum256([]byte(rawSecret))
	tokenHash := hex.EncodeToString(hash[:])
	
	// Create a safe prefix for UI identification
	prefix := rawSecret[:8] // e.g. cpk_xxxx

	key := &domain.AgentKey{
		KeyID:          "key_" + uuid.New().String(),
		OrganizationID: orgID,
		CreatedBy:      userID,
		Name:           req.Name,
		TokenHash:      tokenHash,
		TokenPrefix:    prefix,
		MaxBindings:    req.MaxBindings,
		ExpiresAt:      req.ExpiresAt,
	}

	if err := s.repo.Create(ctx, key); err != nil {
		return nil, "", err
	}

	// Return the domain entity and the raw secret (ONCE)
	return key, rawSecret, nil
}

func (s *agentKeyService) ListKeys(ctx context.Context, orgID string) ([]*domain.AgentKey, error) {
	return s.repo.List(ctx, orgID)
}

func (s *agentKeyService) GetKey(ctx context.Context, orgID, keyID string) (*domain.AgentKey, error) {
	key, err := s.repo.GetByID(ctx, orgID, keyID)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, ErrNotFound
	}
	return key, nil
}

func (s *agentKeyService) UpdateKey(ctx context.Context, orgID, keyID string, req UpdateKeyRequest) (*domain.AgentKey, error) {
	if req.UpdateMaxBindings && req.MaxBindings != nil && *req.MaxBindings <= 0 {
		return nil, ErrInvalidParams
	}
	if req.Name != nil && len(*req.Name) == 0 {
		return nil, ErrInvalidParams
	}
	
	key, err := s.repo.Update(ctx, orgID, keyID, req.Name, req.MaxBindings, req.UpdateMaxBindings, req.ExpiresAt, req.UpdateExpiresAt)
	if err != nil {
		return nil, err
	}
	if key == nil {
		return nil, ErrNotFound
	}
	return key, nil
}

func (s *agentKeyService) RevokeKey(ctx context.Context, orgID, keyID string) error {
	err := s.repo.Revoke(ctx, orgID, keyID)
	if err != nil {
		if err.Error() == "not found or already revoked" {
			return ErrNotFound
		}
		return err
	}
	return nil
}
