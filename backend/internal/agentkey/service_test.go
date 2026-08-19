package agentkey

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

type mockRepo struct {
	keys map[string]*domain.AgentKey
}

func (m *mockRepo) Create(ctx context.Context, key *domain.AgentKey) error {
	m.keys[key.KeyID] = key
	return nil
}

func (m *mockRepo) GetByID(ctx context.Context, orgID, keyID string) (*domain.AgentKey, error) {
	k, ok := m.keys[keyID]
	if !ok || k.OrganizationID != orgID {
		return nil, nil // Not found
	}
	return k, nil
}

func (m *mockRepo) List(ctx context.Context, orgID string) ([]*domain.AgentKey, error) {
	var res []*domain.AgentKey
	for _, k := range m.keys {
		if k.OrganizationID == orgID {
			res = append(res, k)
		}
	}
	return res, nil
}

func (m *mockRepo) Update(ctx context.Context, orgID, keyID string, name string, maxBindings *int, expiresAt *time.Time) (*domain.AgentKey, error) {
	k, ok := m.keys[keyID]
	if !ok || k.OrganizationID != orgID {
		return nil, nil // Not found
	}
	k.Name = name
	k.MaxBindings = maxBindings
	k.ExpiresAt = expiresAt
	k.UpdatedAt = time.Now()
	return k, nil
}

func (m *mockRepo) Revoke(ctx context.Context, orgID, keyID string) error {
	k, ok := m.keys[keyID]
	if !ok || k.OrganizationID != orgID || k.RevokedAt != nil {
		return errors.New("not found or already revoked")
	}
	now := time.Now()
	k.RevokedAt = &now
	k.UpdatedAt = now
	return nil
}

func TestAgentKeyService_CreateKey(t *testing.T) {
	repo := &mockRepo{keys: make(map[string]*domain.AgentKey)}
	svc := NewService(repo)

	ctx := context.Background()
	orgID := "org_1"
	userID := "usr_1"

	req := CreateKeyRequest{
		Name:        "Test Key",
		MaxBindings: nil, // Unlimited
	}

	key, rawSecret, err := svc.CreateKey(ctx, orgID, userID, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Security: raw key stored nowhere
	if key.TokenHash == rawSecret {
		t.Errorf("token hash should not equal raw secret")
	}
	if strings.Contains(key.TokenHash, rawSecret) {
		t.Errorf("token hash should not contain raw secret")
	}

	// Security: safe prefix
	if !strings.HasPrefix(rawSecret, key.TokenPrefix) {
		t.Errorf("raw secret %s should start with prefix %s", rawSecret, key.TokenPrefix)
	}
	if len(key.TokenPrefix) > 12 {
		t.Errorf("token prefix is too long, could leak secret")
	}

	// Invalid params
	invalidMax := 0
	_, _, err = svc.CreateKey(ctx, orgID, userID, CreateKeyRequest{MaxBindings: &invalidMax})
	if !errors.Is(err, ErrInvalidParams) {
		t.Errorf("expected ErrInvalidParams for max_bindings=0, got %v", err)
	}
}

func TestAgentKeyService_TenantIsolation(t *testing.T) {
	repo := &mockRepo{keys: make(map[string]*domain.AgentKey)}
	svc := NewService(repo)
	ctx := context.Background()

	orgA := "org_A"
	orgB := "org_B"

	keyA, _, _ := svc.CreateKey(ctx, orgA, "u1", CreateKeyRequest{Name: "A"})

	// orgB attempts to get orgA's key
	_, err := svc.GetKey(ctx, orgB, keyA.KeyID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("orgB should not find orgA key, got %v", err)
	}

	// orgB attempts to update orgA's key
	_, err = svc.UpdateKey(ctx, orgB, keyA.KeyID, UpdateKeyRequest{Name: "Hacked"})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("orgB should not update orgA key, got %v", err)
	}

	// orgB attempts to revoke orgA's key
	err = svc.RevokeKey(ctx, orgB, keyA.KeyID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("orgB should not revoke orgA key, got %v", err)
	}
}

func TestAgentKeyService_Revoke(t *testing.T) {
	repo := &mockRepo{keys: make(map[string]*domain.AgentKey)}
	svc := NewService(repo)
	ctx := context.Background()

	key, _, _ := svc.CreateKey(ctx, "org1", "u1", CreateKeyRequest{Name: "K"})
	
	err := svc.RevokeKey(ctx, "org1", key.KeyID)
	if err != nil {
		t.Fatalf("unexpected error revoking key: %v", err)
	}

	// Revoked key remains queryable
	fetched, err := svc.GetKey(ctx, "org1", key.KeyID)
	if err != nil {
		t.Fatalf("revoked key should be queryable")
	}
	if fetched.RevokedAt == nil {
		t.Errorf("expected RevokedAt to be set")
	}

	// Double revoke returns ErrNotFound (or already revoked)
	err = svc.RevokeKey(ctx, "org1", key.KeyID)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound for double revoke, got %v", err)
	}
}
