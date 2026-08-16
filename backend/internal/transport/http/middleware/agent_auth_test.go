package middleware_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	"github.com/tcandt/cloudphone-farm/backend/internal/transport/http/middleware"
)

type mockEnrollRepo struct {
	agent *domain.DeviceAgent
}

func (m *mockEnrollRepo) GetAgentByID(ctx context.Context, agentID string) (*domain.DeviceAgent, error) {
	if m.agent != nil && m.agent.AgentID == agentID {
		return m.agent, nil
	}
	return nil, fmt.Errorf("agent not found")
}

func signRequest(pub ed25519.PublicKey, priv ed25519.PrivateKey, method, path string, body []byte, ts int64, nonce string) (string, string) {
	bodyHash := sha256.Sum256(body)
	bodyHashHex := hex.EncodeToString(bodyHash[:])
	canonicalMsg := fmt.Sprintf("%s\n%s\n%s\n%d\n%s", method, path, bodyHashHex, ts, nonce)
	sig := ed25519.Sign(priv, []byte(canonicalMsg))
	return base64.StdEncoding.EncodeToString(sig), canonicalMsg
}

func TestAgentAuthMiddleware_ReplayAndClockSkew(t *testing.T) {
	s := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	agentID := "agt_test_12345"
	mockRepo := &mockEnrollRepo{
		agent: &domain.DeviceAgent{
			AgentID:        agentID,
			OrganizationID: "org_test",
			DeviceID:       "dev_test",
			PublicKey:      pub,
			Status:         "active",
		},
	}

	authMw := middleware.NewAgentAuthMiddleware(mockRepo, rdb)
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	handler := authMw.Handler(nextHandler)

	t.Run("Valid signature within clock window passes", func(t *testing.T) {
		now := time.Now().Unix()
		nonce := "nonce_valid_001"
		sig, _ := signRequest(pub, priv, "GET", "/agent/v1/connect", nil, now, nonce)

		req := httptest.NewRequest("GET", "/agent/v1/connect", nil)
		req.Header.Set("X-Agent-ID", agentID)
		req.Header.Set("X-Agent-Timestamp", strconv.FormatInt(now, 10))
		req.Header.Set("X-Agent-Nonce", nonce)
		req.Header.Set("X-Agent-Signature", sig)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("Replay attack with same nonce is immediately rejected", func(t *testing.T) {
		now := time.Now().Unix()
		nonce := "nonce_replay_002"
		sig, _ := signRequest(pub, priv, "GET", "/agent/v1/connect", nil, now, nonce)

		// First attempt
		req1 := httptest.NewRequest("GET", "/agent/v1/connect", nil)
		req1.Header.Set("X-Agent-ID", agentID)
		req1.Header.Set("X-Agent-Timestamp", strconv.FormatInt(now, 10))
		req1.Header.Set("X-Agent-Nonce", nonce)
		req1.Header.Set("X-Agent-Signature", sig)

		rr1 := httptest.NewRecorder()
		handler.ServeHTTP(rr1, req1)
		assert.Equal(t, http.StatusOK, rr1.Code)

		// Replay attempt with exact same headers
		req2 := httptest.NewRequest("GET", "/agent/v1/connect", nil)
		req2.Header.Set("X-Agent-ID", agentID)
		req2.Header.Set("X-Agent-Timestamp", strconv.FormatInt(now, 10))
		req2.Header.Set("X-Agent-Nonce", nonce)
		req2.Header.Set("X-Agent-Signature", sig)

		rr2 := httptest.NewRecorder()
		handler.ServeHTTP(rr2, req2)
		assert.Equal(t, http.StatusForbidden, rr2.Code)
		assert.Contains(t, rr2.Body.String(), "REPLAY_ATTACK_DETECTED")
	})

	t.Run("Timestamp older than 300s rejected", func(t *testing.T) {
		staleTs := time.Now().Unix() - 305
		nonce := "nonce_stale_003"
		sig, _ := signRequest(pub, priv, "GET", "/agent/v1/connect", nil, staleTs, nonce)

		req := httptest.NewRequest("GET", "/agent/v1/connect", nil)
		req.Header.Set("X-Agent-ID", agentID)
		req.Header.Set("X-Agent-Timestamp", strconv.FormatInt(staleTs, 10))
		req.Header.Set("X-Agent-Nonce", nonce)
		req.Header.Set("X-Agent-Signature", sig)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "TIMESTAMP_OUT_OF_BOUNDS")
	})

	t.Run("Timestamp newer than 300s rejected", func(t *testing.T) {
		futureTs := time.Now().Unix() + 305
		nonce := "nonce_future_004"
		sig, _ := signRequest(pub, priv, "GET", "/agent/v1/connect", nil, futureTs, nonce)

		req := httptest.NewRequest("GET", "/agent/v1/connect", nil)
		req.Header.Set("X-Agent-ID", agentID)
		req.Header.Set("X-Agent-Timestamp", strconv.FormatInt(futureTs, 10))
		req.Header.Set("X-Agent-Nonce", nonce)
		req.Header.Set("X-Agent-Signature", sig)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "TIMESTAMP_OUT_OF_BOUNDS")
	})

	t.Run("Invalid signature rejected", func(t *testing.T) {
		now := time.Now().Unix()
		nonce := "nonce_invalid_005"
		_, wrongPriv, _ := ed25519.GenerateKey(rand.Reader)
		sig, _ := signRequest(pub, wrongPriv, "GET", "/agent/v1/connect", nil, now, nonce)

		req := httptest.NewRequest("GET", "/agent/v1/connect", nil)
		req.Header.Set("X-Agent-ID", agentID)
		req.Header.Set("X-Agent-Timestamp", strconv.FormatInt(now, 10))
		req.Header.Set("X-Agent-Nonce", nonce)
		req.Header.Set("X-Agent-Signature", sig)

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		assert.Contains(t, rr.Body.String(), "SIGNATURE_VERIFICATION_FAILED")
	})
}
