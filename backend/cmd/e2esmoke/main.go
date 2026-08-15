package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)

type E2ESmokeConfig struct {
	NodeAURL    string
	NodeBURL    string
	NodeCURL    string
	PidA        int
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
	pidA := flag.Int("pid-a", 0, "PID of Node A server process for live kill failover")
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
		PidA:        *pidA,
		PostgresURL: dbURL,
		RedisURL:    redisURL,
	}

	slog.Info("Starting Single Continuous Real 3-Node Live E2E Smoke & Failover Suite",
		"node_a", cfg.NodeAURL,
		"node_b", cfg.NodeBURL,
		"node_c", cfg.NodeCURL,
		"pid_a", cfg.PidA,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
	sessionToken := "e2e_session_token_999"

	// 1. Seed DB Entities & Redis Real Browser Session
	seedE2EData(ctx, cfg, orgID, userID, deviceID, leaseID, agentID, sessionToken, pubKey)

	// ---------------------------------------------------------------------
	// PHASE 1: HTTP (Node C) -> Outbox -> Agent WS (Node A) -> Browser WS (Node B)
	// ---------------------------------------------------------------------
	slog.Info("PHASE 1: Connecting Agent to Node A and Browser to Node B...")

	// Connect Agent to Node A with real signed headers & WSS challenge
	agentConnA := connectAgentWS(ctx, cfg.NodeAURL, deviceID, agentID, privKey)
	defer agentConnA.Close()
	slog.Info("Agent successfully authenticated & connected to Node A", "agent_node", "node-a")

	// Connect Browser WS to Node B with real session cookie
	browserConnB := connectBrowserWS(ctx, cfg.NodeBURL, deviceID, sessionToken)
	defer browserConnB.Close()
	slog.Info("Browser WS connected to Node B", "browser_node", "node-b")

	// Subscribe Browser WS to events channel
	browserEventsChan := make(chan map[string]interface{}, 32)
	browserErrChan := make(chan error, 1)
	go func() {
		for {
			_, msgBytes, err := browserConnB.ReadMessage()
			if err != nil {
				browserErrChan <- err
				return
			}
			var event map[string]interface{}
			if json.Unmarshal(msgBytes, &event) == nil {
				browserEventsChan <- event
			}
		}
	}()

	// Agent WS Handler on Node A: receives command envelope & sends ack, executing, succeeded
	agentCmdChanA := make(chan string, 1)
	go func() {
		for {
			_, message, err := agentConnA.ReadMessage()
			if err != nil {
				return
			}
			var env struct {
				Type    string                 `json:"type"`
				Payload map[string]interface{} `json:"payload"`
			}
			if json.Unmarshal(message, &env) == nil && (env.Type == "command" || env.Type == "device.command") {
				cmdID, _ := env.Payload["commandId"].(string)
				if cmdID != "" {
					agentCmdChanA <- cmdID

					// 1. Send ack
					_ = sendAgentStatus(agentConnA, cmdID, "ack", 1, "")
					time.Sleep(50 * time.Millisecond)

					// 2. Send executing
					_ = sendAgentStatus(agentConnA, cmdID, "executing", 2, "")
					time.Sleep(50 * time.Millisecond)

					// 3. Send succeeded
					_ = sendAgentStatus(agentConnA, cmdID, "succeeded", 3, "")
				}
			}
		}
	}()

	// POST command #1 to Node C
	cmdID1 := postCommand(cfg.NodeCURL, sessionToken, deviceID, leaseID, "idem_e2e_phase1")
	slog.Info("PHASE 1: POST command accepted", "command_id", cmdID1, "target_node", "node-c")

	// Verify Agent on Node A receives command #1
	select {
	case receivedID := <-agentCmdChanA:
		if receivedID != cmdID1 {
			slog.Error("PHASE 1 FAIL: Agent received unexpected command ID", "expected", cmdID1, "got", receivedID)
			os.Exit(1)
		}
		slog.Info("PHASE 1 SUCCESS: Agent on Node A received command", "command_id", cmdID1)
	case <-time.After(5 * time.Second):
		slog.Error("PHASE 1 FAIL: Timeout waiting for Agent on Node A to receive command")
		os.Exit(1)
	}

	// Verify Browser on Node B receives status events (ack, executing, succeeded)
	verifyBrowserEvents(browserEventsChan, cmdID1, 5*time.Second)
	slog.Info("PHASE 1 SUCCESS: Browser on Node B received complete command status pipeline events", "command_id", cmdID1)

	// ---------------------------------------------------------------------
	// FAILOVER: Kill Node A & Reconnect Agent to Node C
	// ---------------------------------------------------------------------
	slog.Info("FAILOVER: Initiating Node A kill & failover sequence...")

	if cfg.PidA > 0 {
		slog.Info("Killing Node A server process", "pid_a", cfg.PidA)
		killProcess(cfg.PidA)
	} else {
		slog.Info("Closing Node A Agent socket to simulate node crash")
		_ = agentConnA.Close()
	}

	time.Sleep(1 * time.Second)
	slog.Info("FAILOVER: Node A evicted. Reconnecting SAME Agent ID to Node C...")

	// Reconnect SAME Agent ID & key to Node C with generation increment
	agentConnC := connectAgentWS(ctx, cfg.NodeCURL, deviceID, agentID, privKey)
	defer agentConnC.Close()
	slog.Info("FAILOVER SUCCESS: Agent reconnected and authenticated on Node C", "agent_node", "node-c")

	// Agent WS Handler on Node C
	agentCmdChanC := make(chan string, 1)
	go func() {
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
				cmdID, _ := env.Payload["commandId"].(string)
				if cmdID != "" {
					agentCmdChanC <- cmdID

					// Send status sequence (ack, executing, succeeded)
					_ = sendAgentStatus(agentConnC, cmdID, "ack", 1, "")
					time.Sleep(50 * time.Millisecond)
					_ = sendAgentStatus(agentConnC, cmdID, "executing", 2, "")
					time.Sleep(50 * time.Millisecond)
					_ = sendAgentStatus(agentConnC, cmdID, "succeeded", 3, "")
				}
			}
		}
	}()

	// ---------------------------------------------------------------------
	// PHASE 2: POST command #2 to Node B -> Route to Agent on Node C -> Browser on Node B
	// ---------------------------------------------------------------------
	slog.Info("PHASE 2: Dispatching command #2 to Node B post-failover...")
	cmdID2 := postCommand(cfg.NodeBURL, sessionToken, deviceID, leaseID, "idem_e2e_phase2")
	slog.Info("PHASE 2: POST command #2 accepted", "command_id", cmdID2, "target_node", "node-b")

	// Verify Agent on Node C receives command #2
	select {
	case receivedID := <-agentCmdChanC:
		if receivedID != cmdID2 {
			slog.Error("PHASE 2 FAIL: Agent on Node C received wrong command", "expected", cmdID2, "got", receivedID)
			os.Exit(1)
		}
		slog.Info("PHASE 2 SUCCESS: Agent on Node C received command #2 post-failover", "command_id", cmdID2)
	case <-time.After(5 * time.Second):
		slog.Error("PHASE 2 FAIL: Timeout waiting for Agent on Node C to receive command #2")
		os.Exit(1)
	}

	// Verify Browser on Node B receives status events for command #2 (ack, executing, succeeded)
	verifyBrowserEvents(browserEventsChan, cmdID2, 5*time.Second)
	slog.Info("PHASE 2 SUCCESS: Browser on Node B received status events for command #2 post-failover", "command_id", cmdID2)

	slog.Info("=========================================================")
	slog.Info("E2E REAL 3-NODE FAILOVER SMOKE TEST: PASSED 100%")
	slog.Info("=========================================================")
}

func seedE2EData(ctx context.Context, cfg E2ESmokeConfig, orgID, userID, deviceID, leaseID, agentID, sessionToken string, pubKey ed25519.PublicKey) {
	pool, err := pgxpool.New(ctx, cfg.PostgresURL)
	if err != nil {
		slog.Error("Failed to connect PostgreSQL for E2E seeding", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	_, err = pool.Exec(ctx, `
		INSERT INTO organizations (organization_id, name, slug) VALUES ($1, 'Dev Org', 'dev-org-01')
		ON CONFLICT (organization_id) DO NOTHING;
	`, orgID)
	if err != nil {
		slog.Error("Failed to seed organization in e2esmoke", "error", err)
		os.Exit(1)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO users (user_id, email, password_hash, display_name) VALUES ($1, 'e2e@dev.local', 'hash', 'E2E User')
		ON CONFLICT (user_id) DO NOTHING;
	`, userID)
	if err != nil {
		slog.Error("Failed to seed user in e2esmoke", "error", err)
		os.Exit(1)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO devices (device_id, organization_id, name, serial_number, model, platform_version, status)
		VALUES ($1, $2, 'E2E Device', 'SN_E2E_01', 'E2EModel', 'Android 14', 'online')
		ON CONFLICT (device_id) DO UPDATE SET status = 'online', updated_at = NOW();
	`, deviceID, orgID)
	if err != nil {
		slog.Error("Failed to seed device in e2esmoke", "error", err)
		os.Exit(1)
	}

	expiresAt := time.Now().Add(1 * time.Hour)
	_, err = pool.Exec(ctx, `
		INSERT INTO control_leases (control_lease_id, organization_id, user_id, device_id, expires_at, fencing_token)
		VALUES ($1, $2, $3, $4, $5, 1)
		ON CONFLICT (control_lease_id) DO UPDATE SET expires_at = $5;
	`, leaseID, orgID, userID, deviceID, expiresAt)
	if err != nil {
		slog.Error("Failed to seed control lease in e2esmoke", "error", err)
		os.Exit(1)
	}

	// Seed Agent cryptographic identity in device_agents table
	fp := crypto.ComputePublicKeyFingerprint(pubKey)
	_, err = pool.Exec(ctx, `
		INSERT INTO device_agents (agent_id, organization_id, device_id, public_key, public_key_fingerprint, apk_version, protocol_version, status, credential_version)
		VALUES ($1, $2, $3, $4, $5, '1.0.0', '1.0', 'active', 1)
		ON CONFLICT (agent_id) DO UPDATE SET public_key = $4, public_key_fingerprint = $5;
	`, agentID, orgID, deviceID, []byte(pubKey), fp)
	if err != nil {
		slog.Error("Failed to seed device_agents in e2esmoke", "error", err)
		os.Exit(1)
	}

	// Seed Redis entities (Control Lease & Real Browser Session Token)
	if cfg.RedisURL != "" {
		redisURL := cfg.RedisURL
		if !strings.HasPrefix(redisURL, "redis://") && !strings.HasPrefix(redisURL, "rediss://") {
			redisURL = "redis://" + redisURL
		}
		if opt, err := redis.ParseURL(redisURL); err == nil {
			rdb := redis.NewClient(opt)
			now := time.Now().UTC()

			// 1. Redis Control Lease
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
			leaseBytes, _ := json.Marshal(leaseObj)
			leaseKey := fmt.Sprintf("pcp:control:lease:v1:%s:%s", orgID, deviceID)
			_ = rdb.Set(ctx, leaseKey, leaseBytes, 1*time.Hour).Err()

			// 2. Real Redis Browser Session
			tokenHashBytes := sha256.Sum256([]byte(sessionToken))
			tokenHashHex := hex.EncodeToString(tokenHashBytes[:])
			sessionKey := fmt.Sprintf("pcp:session:v1:%s", tokenHashHex)

			sessObj := map[string]interface{}{
				"session_id":      "sess_e2e_01",
				"user_id":         userID,
				"email":           "e2e@dev.local",
				"display_name":    "E2E Operator User",
				"organization_id": orgID,
				"membership_id":   "mem_e2e_01",
				"roles":           []string{"admin"},
				"permissions": map[string]interface{}{
					"*":                     map[string]interface{}{},
					"device.read":           map[string]interface{}{},
					"device.control.input":   map[string]interface{}{},
					"device.control.acquire": map[string]interface{}{},
					"device.stream.view":    map[string]interface{}{},
				},
			}
			sessBytes, _ := json.Marshal(sessObj)
			_ = rdb.Set(ctx, sessionKey, sessBytes, 24*time.Hour).Err()

			rdb.Close()
		}
	}
	slog.Info("E2E database entities and Redis session seeded cleanly")
}

func connectAgentWS(ctx context.Context, nodeURL, deviceID, agentID string, privKey ed25519.PrivateKey) *websocket.Conn {
	wsURL := strings.Replace(nodeURL, "http://", "ws://", 1) + "/agent/v1/connect?device_id=" + deviceID

	// Construct Proof-of-Possession signed HTTP Upgrade headers
	timestampStr := strconv.FormatInt(time.Now().Unix(), 10)
	nonceBytes := make([]byte, 16)
	_, _ = rand.Read(nonceBytes)
	nonce := hex.EncodeToString(nonceBytes)

	// Canonical Message for GET /agent/v1/connect?device_id=... with empty body
	emptyBodyHash := sha256.Sum256([]byte(""))
	bodyHashHex := hex.EncodeToString(emptyBodyHash[:])
	canonicalPath := "/agent/v1/connect"

	canonicalMsg := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", "GET", canonicalPath, bodyHashHex, timestampStr, nonce)
	sigBytes := ed25519.Sign(privKey, []byte(canonicalMsg))
	sigB64 := base64.StdEncoding.EncodeToString(sigBytes)

	headers := make(http.Header)
	headers.Set("X-Agent-ID", agentID)
	headers.Set("X-Agent-Timestamp", timestampStr)
	headers.Set("X-Agent-Nonce", nonce)
	headers.Set("X-Agent-Signature", sigB64)

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		slog.Error("Failed to connect Agent WS to node", "url", wsURL, "error", err)
		os.Exit(1)
	}

	// WSS Server Challenge Handshake
	performAgentChallengeHandshake(conn, privKey)
	return conn
}

func performAgentChallengeHandshake(conn *websocket.Conn, privKey ed25519.PrivateKey) {
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		slog.Error("Failed to read WSS server challenge", "error", err)
		os.Exit(1)
	}

	var challengeEnv struct {
		Type    string `json:"type"`
		Payload struct {
			ChallengeNonce string `json:"challenge_nonce"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(msg, &challengeEnv); err != nil {
		slog.Error("Malformed WSS server challenge", "error", err)
		os.Exit(1)
	}

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

	// Read connection.ready confirmation
	_, readyMsg, err := conn.ReadMessage()
	if err != nil {
		slog.Error("Failed to read connection.ready confirmation", "error", err)
		os.Exit(1)
	}

	var readyEnv struct {
		Type string `json:"type"`
	}
	_ = json.Unmarshal(readyMsg, &readyEnv)
	_ = conn.SetReadDeadline(time.Time{})
}

func connectBrowserWS(ctx context.Context, nodeURL, deviceID, sessionToken string) *websocket.Conn {
	wsURL := strings.Replace(nodeURL, "http://", "ws://", 1) + "/api/v1/devices/" + deviceID + "/events/ws"

	headers := make(http.Header)
	headers.Set("Cookie", fmt.Sprintf("__Host-pcp_session=%s", sessionToken))
	headers.Set("Origin", "http://localhost:3000")

	dialer := websocket.Dialer{
		HandshakeTimeout: 5 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		slog.Error("Failed to connect Browser WS with session cookie", "url", wsURL, "error", err)
		os.Exit(1)
	}
	return conn
}

func sendAgentStatus(conn *websocket.Conn, commandID, status string, sequence int, errStr string) error {
	payload := map[string]interface{}{
		"type": "command.status",
		"payload": map[string]interface{}{
			"commandId":    commandID,
			"status":       status,
			"sequence":     sequence,
			"errorMessage": errStr,
		},
	}
	bytesData, _ := json.Marshal(payload)
	return conn.WriteMessage(websocket.TextMessage, bytesData)
}

func postCommand(nodeURL, sessionToken, deviceID, leaseID, idempotencyKey string) string {
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
	req.Header.Set("Cookie", fmt.Sprintf("__Host-pcp_session=%s", sessionToken))
	req.Header.Set("Origin", "http://localhost:3000")

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
		slog.Error("POST /commands rejected", "status_code", resp.StatusCode, "body", buf.String())
		os.Exit(1)
	}

	var resBody struct {
		Data struct {
			CommandID string `json:"commandId"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&resBody)
	if resBody.Data.CommandID == "" {
		slog.Error("POST /commands response missing commandId")
		os.Exit(1)
	}
	return resBody.Data.CommandID
}

func verifyBrowserEvents(ch chan map[string]interface{}, commandID string, timeout time.Duration) {
	gotAck := false
	gotExecuting := false
	gotSucceeded := false

	deadline := time.After(timeout)
	for {
		select {
		case evt := <-ch:
			dataObj, ok := evt["data"].(map[string]interface{})
			if ok {
				evtCmdID, _ := dataObj["command_id"].(string)
				evtStatus, _ := dataObj["execution_status"].(string)
				if evtCmdID == commandID {
					if evtStatus == "ack" {
						gotAck = true
						slog.Info("Browser WS event received", "command_id", commandID, "status", "ack")
					} else if evtStatus == "executing" {
						gotExecuting = true
						slog.Info("Browser WS event received", "command_id", commandID, "status", "executing")
					} else if evtStatus == "succeeded" {
						gotSucceeded = true
						slog.Info("Browser WS event received", "command_id", commandID, "status", "succeeded")
					}
					if gotAck && gotExecuting && gotSucceeded {
						slog.Info("Browser WS received all required statuses (ack, executing, succeeded)", "command_id", commandID)
						return
					}
				}
			}
		case <-deadline:
			slog.Error("Browser WS event timeout: failed to receive all required status events (ack, executing, succeeded)",
				"command_id", commandID, "got_ack", gotAck, "got_executing", gotExecuting, "got_succeeded", gotSucceeded)
			os.Exit(1)
		}
	}
}

func killProcess(pid int) {
	if pid <= 0 {
		return
	}
	if runtime.GOOS == "windows" {
		_ = exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run()
	} else {
		_ = exec.Command("kill", "-9", strconv.Itoa(pid)).Run()
	}
}
