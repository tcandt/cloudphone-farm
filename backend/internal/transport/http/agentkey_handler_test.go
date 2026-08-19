package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentkey"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	httptransport "github.com/tcandt/cloudphone-farm/backend/internal/transport/http"
)

type MockAgentKeyService struct {
	mock.Mock
}

func (m *MockAgentKeyService) CreateKey(ctx context.Context, orgID, userID string, req agentkey.CreateKeyRequest) (*domain.AgentKey, string, error) {
	args := m.Called(ctx, orgID, userID, req)
	if args.Get(0) == nil {
		return nil, "", args.Error(2)
	}
	return args.Get(0).(*domain.AgentKey), args.String(1), args.Error(2)
}

func (m *MockAgentKeyService) ListKeys(ctx context.Context, orgID string) ([]*domain.AgentKey, error) {
	args := m.Called(ctx, orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AgentKey), args.Error(1)
}

func (m *MockAgentKeyService) GetKey(ctx context.Context, orgID, keyID string) (*domain.AgentKey, error) {
	args := m.Called(ctx, orgID, keyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AgentKey), args.Error(1)
}

func (m *MockAgentKeyService) UpdateKey(ctx context.Context, orgID, keyID string, req agentkey.UpdateKeyRequest) (*domain.AgentKey, error) {
	args := m.Called(ctx, orgID, keyID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AgentKey), args.Error(1)
}

func (m *MockAgentKeyService) RevokeKey(ctx context.Context, orgID, keyID string) error {
	args := m.Called(ctx, orgID, keyID)
	return args.Error(0)
}

func setupTestRouter(svc *MockAgentKeyService, orgID string) *chi.Mux {
	r := chi.NewRouter()
	
	// mock auth middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal := &auth.Principal{
				UserID:         "user1",
				OrganizationID: orgID,
			}
			ctx := auth.WithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	handler := httptransport.NewAgentKeyHandler(svc)

	r.Route("/api/v2/agent-keys", func(r chi.Router) {
		r.Post("/", handler.Create)
		r.Get("/", handler.List)
		r.Get("/{keyId}", handler.GetByID)
		r.Patch("/{keyId}", handler.Update)
		r.Delete("/{keyId}", handler.Revoke)
	})

	return r
}

func TestAgentKeyHandler_Create_ReturnsRawSecret(t *testing.T) {
	svc := new(MockAgentKeyService)
	r := setupTestRouter(svc, "org1")

	expectedKey := &domain.AgentKey{
		KeyID:          "key1",
		OrganizationID: "org1",
		Name:           "Test Key",
		TokenPrefix:    "cpk_test",
		TokenHash:      "hash",
	}

	svc.On("CreateKey", mock.Anything, "org1", "user1", mock.Anything).Return(expectedKey, "cpk_rawsecret", nil)

	body := []byte(`{"name":"Test Key"}`)
	req, _ := http.NewRequest("POST", "/api/v2/agent-keys", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	
	var res map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &res)
	
	assert.Equal(t, "cpk_rawsecret", res["raw_secret"])
	keyMap := res["key"].(map[string]interface{})
	assert.NotContains(t, keyMap, "token_hash") // token_hash absent
}

func TestAgentKeyHandler_Get_RawSecretAbsent(t *testing.T) {
	svc := new(MockAgentKeyService)
	r := setupTestRouter(svc, "org1")

	expectedKey := &domain.AgentKey{
		KeyID:          "key1",
		OrganizationID: "org1",
		Name:           "Test Key",
		TokenPrefix:    "cpk_test",
		TokenHash:      "hash",
	}

	svc.On("GetKey", mock.Anything, "org1", "key1").Return(expectedKey, nil)

	req, _ := http.NewRequest("GET", "/api/v2/agent-keys/key1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	
	var res map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &res)
	
	assert.NotContains(t, res, "raw_secret")
	assert.NotContains(t, res, "token_hash")
}

func TestAgentKeyHandler_Patch_TriState(t *testing.T) {
	svc := new(MockAgentKeyService)
	r := setupTestRouter(svc, "org1")

	// Case 1: PATCH {"name":"New Name"}
	svc.On("UpdateKey", mock.Anything, "org1", "key1", mock.MatchedBy(func(req agentkey.UpdateKeyRequest) bool {
		return req.Name != nil && *req.Name == "New Name" && !req.UpdateMaxBindings && !req.UpdateExpiresAt
	})).Return(&domain.AgentKey{}, nil).Once()

	body1 := []byte(`{"name":"New Name"}`)
	req1, _ := http.NewRequest("PATCH", "/api/v2/agent-keys/key1", bytes.NewBuffer(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusOK, w1.Code)

	// Case 2: PATCH {"max_bindings":20}
	svc.On("UpdateKey", mock.Anything, "org1", "key1", mock.MatchedBy(func(req agentkey.UpdateKeyRequest) bool {
		return req.Name == nil && req.UpdateMaxBindings && req.MaxBindings != nil && *req.MaxBindings == 20 && !req.UpdateExpiresAt
	})).Return(&domain.AgentKey{}, nil).Once()

	body2 := []byte(`{"max_bindings":20}`)
	req2, _ := http.NewRequest("PATCH", "/api/v2/agent-keys/key1", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)

	// Case 3: PATCH {"max_bindings":null}
	svc.On("UpdateKey", mock.Anything, "org1", "key1", mock.MatchedBy(func(req agentkey.UpdateKeyRequest) bool {
		return req.Name == nil && req.UpdateMaxBindings && req.MaxBindings == nil && !req.UpdateExpiresAt
	})).Return(&domain.AgentKey{}, nil).Once()

	body3 := []byte(`{"max_bindings":null}`)
	req3, _ := http.NewRequest("PATCH", "/api/v2/agent-keys/key1", bytes.NewBuffer(body3))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
	
	// Case 4: PATCH {"expires_at":null}
	svc.On("UpdateKey", mock.Anything, "org1", "key1", mock.MatchedBy(func(req agentkey.UpdateKeyRequest) bool {
		return req.Name == nil && !req.UpdateMaxBindings && req.UpdateExpiresAt && req.ExpiresAt == nil
	})).Return(&domain.AgentKey{}, nil).Once()

	body4 := []byte(`{"expires_at":null}`)
	req4, _ := http.NewRequest("PATCH", "/api/v2/agent-keys/key1", bytes.NewBuffer(body4))
	req4.Header.Set("Content-Type", "application/json")
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusOK, w4.Code)
}

func TestAgentKeyHandler_TenantIsolation(t *testing.T) {
	svc := new(MockAgentKeyService)
	// Router uses org2, but key belongs to org1
	r := setupTestRouter(svc, "org2")

	svc.On("GetKey", mock.Anything, "org2", "key1").Return((*domain.AgentKey)(nil), agentkey.ErrNotFound)

	req, _ := http.NewRequest("GET", "/api/v2/agent-keys/key1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Expected 404 since it's not found in org2
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAgentKeyHandler_Delete(t *testing.T) {
	svc := new(MockAgentKeyService)
	r := setupTestRouter(svc, "org1")

	svc.On("RevokeKey", mock.Anything, "org1", "key1").Return(nil)

	req, _ := http.NewRequest("DELETE", "/api/v2/agent-keys/key1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestAgentKeyHandler_WrongV1Route_Fails(t *testing.T) {
	svc := new(MockAgentKeyService)
	r := setupTestRouter(svc, "org1")

	// We only wired V2 in test router. V1 should 404.
	req, _ := http.NewRequest("GET", "/api/v1/agent-keys", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
