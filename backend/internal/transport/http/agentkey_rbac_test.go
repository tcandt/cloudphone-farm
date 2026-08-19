package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	redisrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
	httptransport "github.com/tcandt/cloudphone-farm/backend/internal/transport/http"
	custommw "github.com/tcandt/cloudphone-farm/backend/internal/transport/http/middleware"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)

func TestAgentKeyHandler_RealRBACRoute(t *testing.T) {
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	sessionRepo := redisrepo.NewSessionRepository(rdb)
	authSvc := auth.NewAuthService(nil, sessionRepo, time.Hour)
	authMw := custommw.NewAuthMiddleware(authSvc, "pcp_session", "production")

	svc := new(MockAgentKeyService)
	svc.On("CreateKey", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&domain.AgentKey{
		KeyID:          "key_1",
		OrganizationID: "org_1",
		CreatedBy:      "usr_1",
	}, "secret", nil).Maybe()

	r := chi.NewRouter()
	r.Use(authMw.Handler)
	r.Use(custommw.TenantMiddleware)

	// In real production, RequirePermission("agent.enroll") is applied by the group
	r.Group(func(r chi.Router) {
		r.Use(custommw.RequirePermission("agent.enroll"))
		handler := httptransport.NewAgentKeyHandler(svc)
		r.Route("/agent-keys", handler.RegisterRoutes)
	})

	// 1. Prepare valid sessions
	validRawToken := "raw_token_enroll"
	validHash := crypto.HashToken(validRawToken)
	err := sessionRepo.CreateSession(context.Background(), validHash, &redisrepo.SessionData{
		SessionID:      "sess_1",
		UserID:         "usr_1",
		OrganizationID: "org_1",
		Permissions:    map[string]struct{}{"agent.enroll": {}},
	}, time.Hour)
	require.NoError(t, err)

	invalidRawToken := "raw_token_read_only"
	invalidHash := crypto.HashToken(invalidRawToken)
	err = sessionRepo.CreateSession(context.Background(), invalidHash, &redisrepo.SessionData{
		SessionID:      "sess_2",
		UserID:         "usr_2",
		OrganizationID: "org_1",
		Permissions:    map[string]struct{}{"agent.read": {}},
	}, time.Hour)
	require.NoError(t, err)

	createPayload := map[string]interface{}{
		"name": "Test Key",
	}
	body, _ := json.Marshal(createPayload)

	// Scenario 1: Unauthenticated -> 401
	reqUnauth := httptest.NewRequest("POST", "/agent-keys/", bytes.NewBuffer(body))
	reqUnauth.Header.Set("Content-Type", "application/json")
	rrUnauth := httptest.NewRecorder()
	r.ServeHTTP(rrUnauth, reqUnauth)
	require.Equal(t, http.StatusUnauthorized, rrUnauth.Code, "Expected 401 for missing session")

	// Scenario 2: Authenticated without agent.enroll -> 403
	reqNoPerm := httptest.NewRequest("POST", "/agent-keys/", bytes.NewBuffer(body))
	reqNoPerm.Header.Set("Content-Type", "application/json")
	reqNoPerm.AddCookie(&http.Cookie{Name: "pcp_session", Value: invalidRawToken})
	rrNoPerm := httptest.NewRecorder()
	r.ServeHTTP(rrNoPerm, reqNoPerm)
	require.Equal(t, http.StatusForbidden, rrNoPerm.Code, "Expected 403 for missing agent.enroll permission")

	// Scenario 3: Authenticated with agent.enroll -> 201 allowed
	reqValid := httptest.NewRequest("POST", "/agent-keys/", bytes.NewBuffer(body))
	reqValid.Header.Set("Content-Type", "application/json")
	reqValid.AddCookie(&http.Cookie{Name: "pcp_session", Value: validRawToken})
	rrValid := httptest.NewRecorder()
	r.ServeHTTP(rrValid, reqValid)
	require.Equal(t, http.StatusCreated, rrValid.Code, "Expected 201 for valid agent.enroll permission")
}
