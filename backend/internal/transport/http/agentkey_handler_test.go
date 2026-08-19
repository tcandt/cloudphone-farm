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
	custommw "github.com/tcandt/cloudphone-farm/backend/internal/transport/http/middleware"
	"github.com/go-chi/cors"
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

func (m *MockAgentKeyService) GetBindings(ctx context.Context, orgID, keyID string) ([]*domain.AgentKeyBinding, error) {
	args := m.Called(ctx, orgID, keyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AgentKeyBinding), args.Error(1)
}

func setupTestRouter(svc *MockAgentKeyService, orgID string) *chi.Mux {
	r := chi.NewRouter()

	// Use real CORS configuration from middleware
	r.Use(cors.Handler(custommw.GetCorsOptions([]string{"http://localhost:3000"})))

	// mock auth middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal := &auth.Principal{
				UserID:         "user1",
				OrganizationID: orgID,
				Permissions: map[string]struct{}{
					"agent.enroll": {},
					"agent.read":   {},
				},
			}
			ctx := auth.WithPrincipal(r.Context(), principal)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	handler := httptransport.NewAgentKeyHandler(svc)

	r.Route("/api/v2", func(r chi.Router) {
		r.Route("/agent-keys", handler.RegisterRoutes)
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
	assert.Equal(t, float64(0), res["active_bindings"])
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

	// Case 5: PATCH malformed type
	body5 := []byte(`{"name":123}`)
	req5, _ := http.NewRequest("PATCH", "/api/v2/agent-keys/key1", bytes.NewBuffer(body5))
	req5.Header.Set("Content-Type", "application/json")
	w5 := httptest.NewRecorder()
	r.ServeHTTP(w5, req5)
	assert.Equal(t, http.StatusBadRequest, w5.Code)

	// Case 6: PATCH fractional quota
	body6 := []byte(`{"max_bindings":1.5}`)
	req6, _ := http.NewRequest("PATCH", "/api/v2/agent-keys/key1", bytes.NewBuffer(body6))
	req6.Header.Set("Content-Type", "application/json")
	w6 := httptest.NewRecorder()
	r.ServeHTTP(w6, req6)
	assert.Equal(t, http.StatusBadRequest, w6.Code)

	// Case 7: PATCH unknown immutable field
	body7 := []byte(`{"token_hash":"x"}`)
	req7, _ := http.NewRequest("PATCH", "/api/v2/agent-keys/key1", bytes.NewBuffer(body7))
	req7.Header.Set("Content-Type", "application/json")
	w7 := httptest.NewRecorder()
	r.ServeHTTP(w7, req7)
	assert.Equal(t, http.StatusBadRequest, w7.Code)

	// Case 8: PATCH name null
	body8 := []byte(`{"name":null}`)
	req8, _ := http.NewRequest("PATCH", "/api/v2/agent-keys/key1", bytes.NewBuffer(body8))
	req8.Header.Set("Content-Type", "application/json")
	w8 := httptest.NewRecorder()
	r.ServeHTTP(w8, req8)
	assert.Equal(t, http.StatusBadRequest, w8.Code)

	// Case 9: PATCH {"organization_id":"other"}
	body9 := []byte(`{"organization_id":"other"}`)
	req9, _ := http.NewRequest("PATCH", "/api/v2/agent-keys/key1", bytes.NewBuffer(body9))
	req9.Header.Set("Content-Type", "application/json")
	w9 := httptest.NewRecorder()
	r.ServeHTTP(w9, req9)
	assert.Equal(t, http.StatusBadRequest, w9.Code)

	// Case 10: PATCH {"revoked_at":null}
	body10 := []byte(`{"revoked_at":null}`)
	req10, _ := http.NewRequest("PATCH", "/api/v2/agent-keys/key1", bytes.NewBuffer(body10))
	req10.Header.Set("Content-Type", "application/json")
	w10 := httptest.NewRecorder()
	r.ServeHTTP(w10, req10)
	assert.Equal(t, http.StatusBadRequest, w10.Code)

	// Case 11: Trailing JSON
	body11 := []byte(`{} {}`)
	req11, _ := http.NewRequest("PATCH", "/api/v2/agent-keys/key1", bytes.NewBuffer(body11))
	req11.Header.Set("Content-Type", "application/json")
	w11 := httptest.NewRecorder()
	r.ServeHTTP(w11, req11)
	assert.Equal(t, http.StatusBadRequest, w11.Code)

	// Case 12: Valid expires_at
	svc.On("UpdateKey", mock.Anything, "org1", "key1", mock.MatchedBy(func(req agentkey.UpdateKeyRequest) bool {
		return req.UpdateExpiresAt && req.ExpiresAt != nil
	})).Return(&domain.AgentKey{}, nil).Once()

	body12 := []byte(`{"expires_at":"2026-08-19T10:00:00Z"}`)
	req12, _ := http.NewRequest("PATCH", "/api/v2/agent-keys/key1", bytes.NewBuffer(body12))
	req12.Header.Set("Content-Type", "application/json")
	w12 := httptest.NewRecorder()
	r.ServeHTTP(w12, req12)
	assert.Equal(t, http.StatusOK, w12.Code)
}

func TestAgentKeyHandler_TenantIsolation(t *testing.T) {
	svc := new(MockAgentKeyService)
	// Router uses org2, but key belongs to org1
	r := setupTestRouter(svc, "org2")

	// GET
	svc.On("GetKey", mock.Anything, "org2", "key1").Return((*domain.AgentKey)(nil), agentkey.ErrNotFound)
	req1, _ := http.NewRequest("GET", "/api/v2/agent-keys/key1", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	assert.Equal(t, http.StatusNotFound, w1.Code)

	// PATCH
	svc.On("UpdateKey", mock.Anything, "org2", "key1", mock.Anything).Return((*domain.AgentKey)(nil), agentkey.ErrNotFound)
	body := []byte(`{"name":"Hacked"}`)
	req2, _ := http.NewRequest("PATCH", "/api/v2/agent-keys/key1", bytes.NewBuffer(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusNotFound, w2.Code)

	// DELETE
	svc.On("RevokeKey", mock.Anything, "org2", "key1").Return(agentkey.ErrNotFound)
	req3, _ := http.NewRequest("DELETE", "/api/v2/agent-keys/key1", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusNotFound, w3.Code)

	// GET Bindings
	svc.On("GetBindings", mock.Anything, "org2", "key1").Return(([]*domain.AgentKeyBinding)(nil), agentkey.ErrNotFound)
	req4, _ := http.NewRequest("GET", "/api/v2/agent-keys/key1/devices", nil)
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusNotFound, w4.Code)
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

func TestAgentKeyHandler_CORS_Options(t *testing.T) {
	svc := new(MockAgentKeyService)
	r := setupTestRouter(svc, "org1")

	req, _ := http.NewRequest("OPTIONS", "/api/v2/agent-keys/key1", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "PATCH")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should be successful preflight
	assert.Equal(t, http.StatusOK, w.Code)

	// Assert methods
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "PATCH")
	
	// Assert origin
	assert.Equal(t, "http://localhost:3000", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestAgentKeyHandler_GetBindings_Success(t *testing.T) {
	svc := new(MockAgentKeyService)
	r := setupTestRouter(svc, "org1")

	expectedBindings := []*domain.AgentKeyBinding{
		{
			BindingID: "b1",
			DeviceID:  "d1",
			AgentID:   "a1",
		},
	}

	svc.On("GetBindings", mock.Anything, "org1", "key1").Return(expectedBindings, nil)

	req, _ := http.NewRequest("GET", "/api/v2/agent-keys/key1/devices", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	
	var res []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &res)
	
	assert.Len(t, res, 1)
	assert.Equal(t, "b1", res[0]["binding_id"])
}

