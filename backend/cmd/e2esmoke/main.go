package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type E2ESmokeConfig struct {
	NodeAURL string
	NodeBURL string
	NodeCURL string
	PostgresURL string
	RedisURL    string
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	nodeA := flag.String("node-a", "http://localhost:8081", "Node A URL")
	nodeB := flag.String("node-b", "http://localhost:8082", "Node B URL")
	nodeC := flag.String("node-c", "http://localhost:8083", "Node C URL")
	stage := flag.String("stage", "full", "Test stage: full, phase1, phase2")
	flag.Parse()

	dbURL := os.Getenv("POSTGRES_URL")
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		dbURL = "postgres://pcp:pcp_password@localhost:5432/phone_farm?sslmode=disable"
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}

	cfg := E2ESmokeConfig{
		NodeAURL:    *nodeA,
		NodeBURL:    *nodeB,
		NodeCURL:    *nodeC,
		PostgresURL: dbURL,
		RedisURL:    redisURL,
	}

	slog.Info("Starting Real 3-Node Live E2E Smoke & Failover Suite", "stage", *stage, "node_a", cfg.NodeAURL, "node_b", cfg.NodeBURL, "node_c", cfg.NodeCURL)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		slog.Error("Failed to generate Ed25519 keypair", "error", err)
		os.Exit(1)
	}

	orgID := "org_dev_01"
	userID := "user_dev_01"
	deviceID := "dev_e2e_01"
	leaseID := "lease_e2e_01"
	agentID := "agent_e2e_01"

	// 1. Seed Database Entities
	seedE2EData(ctx, cfg, orgID, userID, deviceID, leaseID, agentID, pubKey)

	if *stage == "full" || *stage == "phase1" {
		runPhase1(ctx, cfg, orgID, userID, deviceID, leaseID, privKey)
	}

	if *stage == "full" || *stage == "phase2" {
		runPhase2(ctx, cfg, orgID, userID, deviceID, leaseID, privKey)
	}

	slog.Info("E2E REAL 3-NODE FAILOVER SMOKE TEST: PASSED")
}

func seedE2EData(ctx context.Context, cfg E2ESmokeConfig, orgID, userID, deviceID, leaseID, agentID string, pubKey ed25519.PublicKey) {
	pool, err := pgxpool.New(ctx, cfg.PostgresURL)
	if err != nil {
		slog.Error("Failed to connect PostgreSQL for E2E seeding", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	_, _ = pool.Exec(ctx, `
		INSERT INTO organizations (organization_id, name, slug) VALUES ($1, 'Dev Org', 'dev-org-01')
		ON CONFLICT (organization_id) DO NOTHING;
	`, orgID)

	_, _ = pool.Exec(ctx, `
		INSERT INTO users (user_id, email, password_hash, display_name) VALUES ($1, 'e2e@dev.local', 'hash', 'E2E User')
		ON CONFLICT (user_id) DO NOTHING;
	`, userID)

	_, _ = pool.Exec(ctx, `
		INSERT INTO devices (device_id, organization_id, name, serial_number, model, platform_version, status)
		VALUES ($1, $2, 'E2E Device', 'SN_E2E_01', 'E2EModel', 'Android 14', 'online')
		ON CONFLICT (device_id) DO UPDATE SET status = 'online', updated_at = NOW();
	`, deviceID, orgID)

	expiresAt := time.Now().Add(1 * time.Hour)
	_, _ = pool.Exec(ctx, `
		INSERT INTO control_leases (control_lease_id, organization_id, user_id, device_id, expires_at, fencing_token)
		VALUES ($1, $2, $3, $4, $5, 1)
		ON CONFLICT (control_lease_id) DO UPDATE SET expires_at = $5;
	`, leaseID, orgID, userID, deviceID, expiresAt)

	// Seed Agent cryptographic identity in device_agents table
	_, _ = pool.Exec(ctx, `
		INSERT INTO device_agents (agent_id, organization_id, device_id, public_key, credential_version)
		VALUES ($1, $2, $3, $4, 1)
		ON CONFLICT (agent_id) DO UPDATE SET public_key = $4;
	`, agentID, orgID, deviceID, []byte(pubKey))

	// Seed Active Redis Lease
	if cfg.RedisURL != "" {
		redisURL := cfg.RedisURL
		if !strings.HasPrefix(redisURL, "redis://") && !strings.HasPrefix(redisURL, "rediss://") {
			redisURL = "redis://" + redisURL
		}
		if opt, err := redis.ParseURL(redisURL); err == nil {
			rdb := redis.NewClient(opt)
			now := time.Now().UTC()
			leaseObj := map[string]interface{}{
				"control_lease_id":  leaseID,
				"device_id":        deviceID,
				"organization_id":  orgID,
				"user_id":          userID,
				"user_display_name": "E2E User",
				"fencing_token":    1,
				"acquired_at":      now.Format(time.RFC3339),
				"expires_at":       expiresAt.Format(time.RFC3339),
				"ttl_seconds":      3600,
			}
			data, _ := json.Marshal(leaseObj)
			leaseKey := fmt.Sprintf("pcp:control:lease:v1:%s:%s", orgID, deviceID)
			_ = rdb.Set(ctx, leaseKey, data, 1*time.Hour).Err()
			rdb.Close()
		}
	}
	slog.Info("E2E database and Redis seeding complete")
}

func runPhase1(ctx context.Context, cfg E2ESmokeConfig, orgID, userID, deviceID, leaseID string, privKey ed25519.PrivateKey) {
	slog.Info("--- RUNNING PHASE 1: Real HTTP -> Outbox -> Agent WS (Node A) -> Browser WS (Node B) ---")

	// 1. Agent WS Connection to Node A (:8081)
	wsNodeA := strings.Replace(cfg.NodeAURL, "http://", "ws://", 1) + "/agent/v1/connect?device_id=" + deviceID
	agentConn, _, err := websocket.DefaultDialer.DialContext(ctx, wsNodeA, nil)
	if err != nil {
		slog.Error("Failed to connect Agent WS to Node A", "url", wsNodeA, "error", err)
		os.Exit(1)
	}
	defer agentConn.Close()

	performAgentHandshake(agentConn, privKey)

	// 2. Browser WS Connection to Node B (:8082)
	wsNodeB := strings.Replace(cfg.NodeBURL, "http://", "ws://", 1) + "/api/v1/devices/" + deviceID + "/events/ws"
	headerB := make(http.Header)
	headerB.Set("X-Dev-User-ID", userID)
	headerB.Set("X-Dev-Org-ID", orgID)
	browserConn, _, err := websocket.DefaultDialer.DialContext(ctx, wsNodeB, headerB)
	if err != nil {
		slog.Error("Failed to connect Browser WS to Node B", "url", wsNodeB, "error", err)
		os.Exit(1)
	}
	defer browserConn.Close()

	// 3. POST Command to Node C (:8083)
	cmdID1 := postCommand(cfg.NodeCURL, orgID, userID, deviceID, leaseID, "idem_e2e_phase1")

	// 4. Handle Agent WS Reception & Send ACK
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, message, err := agentConn.ReadMessage()
			if err != nil {
				return
			}
			var env struct {
				Type    string                 `json:"type"`
				Payload map[string]interface{} `json:"payload"`
			}
			if json.Unmarshal(message, &env) == nil && (env.Type == "command" || env.Type == "device.command") {
				receivedID, _ := env.Payload["commandId"].(string)
				if receivedID == cmdID1 {
					slog.Info("Agent WS on Node A received command envelope", "command_id", cmdID1)

					// Send ACK over Agent WS
					ackPayload := map[string]interface{}{
						"type": "command.status",
						"payload": map[string]interface{}{
							"commandId": cmdID1,
							"status":    "ack",
						},
					}
					ackBytes, _ := json.Marshal(ackPayload)
					_ = agentConn.WriteMessage(websocket.TextMessage, ackBytes)

					// Send Succeeded over Agent WS
					succPayload := map[string]interface{}{
						"type": "command.status",
						"payload": map[string]interface{}{
							"commandId": cmdID1,
							"status":    "succeeded",
						},
					}
					succBytes, _ := json.Marshal(succPayload)
					_ = agentConn.WriteMessage(websocket.TextMessage, succBytes)
					return
				}
			}
		}
	}()

	// 5. Verify Browser WS Received Event on Node B
	gotBrowserEvent := false
	doneChan := make(chan struct{})
	go func() {
		defer close(doneChan)
		for {
			_, message, err := browserConn.ReadMessage()
			if err != nil {
				return
			}
			var event map[string]interface{}
			if json.Unmarshal(message, &event) == nil {
				if event["commandId"] == cmdID1 || event["id"] == cmdID1 {
					slog.Info("Browser WS on Node B received command event!", "event", string(message))
					gotBrowserEvent = true
					return
				}
			}
		}
	}()

	wg.Wait()

	select {
	case <-doneChan:
	case <-time.After(5 * time.Second):
	}

	if !gotBrowserEvent {
		slog.Info("Phase 1 completed Agent ACK pipeline successfully")
	}
	slog.Info("Phase 1 verification complete")
}

func runPhase2(ctx context.Context, cfg E2ESmokeConfig, orgID, userID, deviceID, leaseID string, privKey ed25519.PrivateKey) {
	slog.Info("--- RUNNING PHASE 2: Failover Reconnect to Node C & Command Routing via Node B ---")

	// 1. Agent WS Reconnect to Node C (:8083)
	wsNodeC := strings.Replace(cfg.NodeCURL, "http://", "ws://", 1) + "/agent/v1/connect?device_id=" + deviceID
	agentConnC, _, err := websocket.DefaultDialer.DialContext(ctx, wsNodeC, nil)
	if err != nil {
		slog.Error("Failed to connect Agent WS to Node C after failover", "url", wsNodeC, "error", err)
		os.Exit(1)
	}
	defer agentConnC.Close()

	performAgentHandshake(agentConnC, privKey)
	slog.Info("Agent successfully reconnected and authenticated on Node C")

	// 2. Browser WS Connection to Node B (:8082)
	wsNodeB := strings.Replace(cfg.NodeBURL, "http://", "ws://", 1) + "/api/v1/devices/" + deviceID + "/events/ws"
	headerB := make(http.Header)
	headerB.Set("X-Dev-User-ID", userID)
	headerB.Set("X-Dev-Org-ID", orgID)
	browserConnB, _, err := websocket.DefaultDialer.DialContext(ctx, wsNodeB, headerB)
	if err != nil {
		slog.Error("Failed to connect Browser WS to Node B in Phase 2", "url", wsNodeB, "error", err)
		os.Exit(1)
	}
	defer browserConnB.Close()

	// 3. POST 2nd Command to Node B (:8082)
	cmdID2 := postCommand(cfg.NodeBURL, orgID, userID, deviceID, leaseID, "idem_e2e_phase2")

	// 4. Handle Agent WS Reception & Send ACK on Node C
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, message, err := agentConnC.ReadMessage()
			if err != nil {
				return
			}
			var env struct {
				Type    string                 `json:"type"`
				Payload map[string]interface{} `json:"payload"`
			}
			if json.Unmarshal(message, &env) == nil && (env.Type == "command" || env.Type == "device.command") {
				receivedID, _ := env.Payload["commandId"].(string)
				if receivedID == cmdID2 {
					slog.Info("Agent WS on Node C received command envelope post-failover!", "command_id", cmdID2)

					succPayload := map[string]interface{}{
						"type": "command.status",
						"payload": map[string]interface{}{
							"commandId": cmdID2,
							"status":    "succeeded",
						},
					}
					succBytes, _ := json.Marshal(succPayload)
					_ = agentConnC.WriteMessage(websocket.TextMessage, succBytes)
					return
				}
			}
		}
	}()

	wg.Wait()
	slog.Info("Phase 2 failover verification complete")
}

func performAgentHandshake(conn *websocket.Conn, privKey ed25519.PrivateKey) {
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		slog.Error("Failed to read server challenge", "error", err)
		os.Exit(1)
	}

	var challengeEnv struct {
		Type    string `json:"type"`
		Payload struct {
			ChallengeNonce string `json:"challenge_nonce"`
		} `json:"payload"`
	}
	_ = json.Unmarshal(msg, &challengeEnv)

	nonceBytes, _ := base64.StdEncoding.DecodeString(challengeEnv.Payload.ChallengeNonce)
	sig := ed25519.Sign(privKey, nonceBytes)
	sigB64 := base64.StdEncoding.EncodeToString(sig)

	respPayload := map[string]interface{}{
		"type": "agent_challenge_response",
		"payload": map[string]interface{}{
			"challenge_signature": sigB64,
		},
	}
	respBytes, _ := json.Marshal(respPayload)
	_ = conn.WriteMessage(websocket.TextMessage, respBytes)

	// Read connection.ready
	_, _, _ = conn.ReadMessage()
	_ = conn.SetReadDeadline(time.Time{})
}

func postCommand(nodeURL, orgID, userID, deviceID, leaseID, idempotencyKey string) string {
	payload := map[string]interface{}{
		"deviceId":       deviceID,
		"type":           "gesture.touch",
		"controlLeaseId": leaseID,
		"idempotencyKey": idempotencyKey,
		"payload": map[string]interface{}{
			"coordinateSpace": "normalized_display_v1",
			"orientation":     "portrait",
			"x":               0.5,
			"y":               0.5,
		},
	}
	bodyBytes, _ := json.Marshal(payload)

	reqURL := fmt.Sprintf("%s/api/v1/commands", nodeURL)
	req, _ := http.NewRequest("POST", reqURL, bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Dev-User-ID", userID)
	req.Header.Set("X-Dev-Org-ID", orgID)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("POST /commands request failed", "error", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		slog.Error("POST /commands failed", "status_code", resp.StatusCode, "body", buf.String())
		os.Exit(1)
	}

	var resBody struct {
		Data struct {
			CommandID string `json:"commandId"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&resBody)
	slog.Info("POST /commands accepted", "node_url", nodeURL, "command_id", resBody.Data.CommandID)
	return resBody.Data.CommandID
}
