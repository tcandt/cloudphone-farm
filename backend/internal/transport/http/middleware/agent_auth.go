package middleware

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	pgrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
)

type agentCtxKey string

const AgentContextKey agentCtxKey = "authenticated_agent"

type AgentAuthMiddleware struct {
	enrollRepo *pgrepo.EnrollmentRepository
	rdb        *redis.Client
}

func NewAgentAuthMiddleware(enrollRepo *pgrepo.EnrollmentRepository, rdb *redis.Client) *AgentAuthMiddleware {
	return &AgentAuthMiddleware{
		enrollRepo: enrollRepo,
		rdb:        rdb,
	}
}

func (m *AgentAuthMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agentID := r.Header.Get("X-Agent-ID")
		timestampStr := r.Header.Get("X-Agent-Timestamp")
		nonce := r.Header.Get("X-Agent-Nonce")
		signatureB64 := r.Header.Get("X-Agent-Signature")

		if agentID == "" || timestampStr == "" || nonce == "" || signatureB64 == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "AGENT_UNAUTHENTICATED",
				"message":   "Agent proof-of-possession headers missing (X-Agent-ID, X-Agent-Timestamp, X-Agent-Nonce, X-Agent-Signature)",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		// 1. Verify Timestamp Window (±60s)
		ts, err := strconv.ParseInt(timestampStr, 10, 64)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "INVALID_TIMESTAMP",
				"message":   "X-Agent-Timestamp must be integer unix seconds",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		nowTs := time.Now().Unix()
		if ts < nowTs-60 || ts > nowTs+60 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "TIMESTAMP_OUT_OF_BOUNDS",
				"message":   "Request timestamp outside of ±60s clock skew window",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		// 2. Replay Protection via Redis Nonce Lock (Strict Fail-Closed Policy)
		if m.rdb == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "SERVICE_UNAVAILABLE",
				"message":   "Replay protection store (Redis) unavailable",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		nonceKey := fmt.Sprintf("pcp:replay:%s:%s", agentID, nonce)
		setOk, err := m.rdb.SetNX(r.Context(), nonceKey, 1, 120*time.Second).Result()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "SERVICE_UNAVAILABLE",
				"message":   "Replay protection store check failed",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}
		if !setOk {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "REPLAY_ATTACK_DETECTED",
				"message":   "Request nonce has already been processed",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		// 3. Load Agent & Public Key
		agent, err := m.enrollRepo.GetAgentByID(r.Context(), agentID)
		if err != nil || len(agent.PublicKey) != ed25519.PublicKeySize {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "AGENT_CREDENTIAL_INVALID",
				"message":   "Agent credential invalid, revoked, or key size malformed",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		// 4. Read & Hash Request Body
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "INVALID_BODY",
				"message":   "Failed to read request body for signature verification",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}
		// Restore r.Body for downstream handlers
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		bodyHash := sha256.Sum256(bodyBytes)
		bodyHashHex := hex.EncodeToString(bodyHash[:])

		// 5. Construct Canonical Signed Message
		canonicalMsg := fmt.Sprintf("%s\n%s\n%s\n%s\n%s",
			r.Method,
			r.URL.Path,
			bodyHashHex,
			timestampStr,
			nonce,
		)

		// Decode Signature
		sigBytes, err := base64.StdEncoding.DecodeString(signatureB64)
		if err != nil {
			sigBytes, err = base64.URLEncoding.DecodeString(signatureB64)
		}
		if err != nil || len(sigBytes) != ed25519.SignatureSize {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "SIGNATURE_MALFORMED",
				"message":   "Ed25519 signature format invalid",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		// 6. Cryptographic Ed25519 Verification
		if !ed25519.Verify(agent.PublicKey, []byte(canonicalMsg), sigBytes) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "SIGNATURE_VERIFICATION_FAILED",
				"message":   "Cryptographic proof-of-possession verification failed",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
			return
		}

		ctx := context.WithValue(r.Context(), AgentContextKey, agent)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
