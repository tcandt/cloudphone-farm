package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)

type GateResult struct {
	Pillar       string `json:"pillar"`
	Name         string `json:"name"`
	Passed       bool   `json:"passed"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type ReleaseGateReport struct {
	Timestamp   string       `json:"timestamp"`
	Nodes       []string     `json:"nodes"`
	TotalGates  int          `json:"total_gates"`
	PassedGates int          `json:"passed_gates"`
	FailedGates int          `json:"failed_gates"`
	Status      string       `json:"status"` // PASSED, FAILED
	Results     []GateResult `json:"results"`
}

func main() {
	nodesFlag := flag.String("nodes", "http://localhost:8081,http://localhost:8082,http://localhost:8083", "Comma-separated backend cluster node URLs")
	caddyFlag := flag.String("caddy-url", "http://localhost:80", "Caddy reverse proxy URL")
	pgURLFlag := flag.String("postgres-url", "postgres://pcp_user:pcp_secure_password_2026@localhost:5432/phone_control_platform?sslmode=disable", "PostgreSQL URL for seeding")
	redisURLFlag := flag.String("redis-url", "redis://localhost:6379/0", "Redis URL for seeding")
	flag.Parse()

	slog.Info("Starting Phase 1.6 Production Deployment & Fleet Readiness Release Gate Automation...")

	nodes := strings.Split(*nodesFlag, ",")
	report := ReleaseGateReport{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Nodes:     nodes,
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, *pgURLFlag)
	if err != nil {
		slog.Error("Failed to connect PostgreSQL for release gate execution", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdbOptions, err := redis.ParseURL(*redisURLFlag)
	if err != nil {
		slog.Error("Failed to parse Redis URL", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(rdbOptions)
	defer rdb.Close()

	var results []GateResult

	// Gate 1: Health & Hardened Readiness Probes across Cluster Nodes
	for i, nodeURL := range nodes {
		g := checkNodeReadiness(nodeURL, i)
		results = append(results, g)
	}

	// Gate 2: Caddy Reverse Proxy & Security Headers
	results = append(results, checkCaddyPerimeter(*caddyFlag))

	// Seed Entities for Real Auth, Revocation & Viewer Tests
	orgID := "org_p16_gate"
	userID := "usr_p16_admin"
	deviceID := "dev_p16_gate_01"
	agentID := "agt_p16_gate_01"

	pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)
	fingerprint := crypto.ComputePublicKeyFingerprint(pubKey)
	sessionToken := "sess_token_p16_admin"

	seedEntities(ctx, pool, rdb, orgID, userID, deviceID, agentID, pubKey, fingerprint, sessionToken)

	// Gate 3: Real Agent Production Auth & WSS Challenge Handshake
	results = append(results, checkRealAgentAuthHandshake(nodes[0], deviceID, agentID, privKey, fingerprint))

	// Gate 4: Real-Time Agent Revocation Teardown Proof (Cross-Node WS Close & Reconnect 401 Rejection)
	results = append(results, checkRealRevocationTeardown(nodes[0], nodes[1], pool, orgID, deviceID, agentID, privKey, fingerprint, sessionToken))

	// Gate 5: Distributed WebRTC Viewer Quota Limit Enforcement (Max 1)
	results = append(results, checkRealViewerQuotaLimit(nodes[1], pool, rdb, orgID, userID, sessionToken))

	// Gate 6: Backup, Restore & Rollback Smoke Test
	results = append(results, checkBackupRestoreSmoke(ctx, pool, *pgURLFlag))

	// Calculate Report Verdict
	report.Results = results
	report.TotalGates = len(results)
	passed := 0
	for _, r := range results {
		if r.Passed {
			passed++
		}
	}
	report.PassedGates = passed
	report.FailedGates = report.TotalGates - passed

	if report.FailedGates == 0 {
		report.Status = "PASSED"
	} else {
		report.Status = "FAILED"
	}

	out, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(out))

	if report.Status != "PASSED" {
		slog.Error("Phase 1.6 Release Gate Automation FAILED", "passed", passed, "total", report.TotalGates)
		os.Exit(1)
	}

	slog.Info("Phase 1.6 Release Gate Automation PASSED 100%", "passed", passed, "total", report.TotalGates)
}

func seedEntities(ctx context.Context, pool *pgxpool.Pool, rdb *redis.Client, orgID, userID, deviceID, agentID string, pubKey []byte, fingerprint, sessionToken string) {
	_, _ = pool.Exec(ctx, `INSERT INTO organizations (organization_id, name, slug) VALUES ($1, 'P16 Org', 'p16-org') ON CONFLICT DO NOTHING;`, orgID)
	_, _ = pool.Exec(ctx, `INSERT INTO users (user_id, email, password_hash, display_name) VALUES ($1, 'admin@p16.local', 'hash', 'P16 Admin') ON CONFLICT DO NOTHING;`, userID)
	_, _ = pool.Exec(ctx, `INSERT INTO devices (device_id, organization_id, name, serial_number, model, platform_version, status) VALUES ($1, $2, 'P16 Device', 'SN_P16_01', 'P16Model', 'Android 15', 'online') ON CONFLICT (organization_id, device_id) DO UPDATE SET status = 'online';`, deviceID, orgID)
	_, _ = pool.Exec(ctx, `INSERT INTO device_agents (agent_id, organization_id, device_id, public_key, public_key_fingerprint, apk_version, protocol_version, status) VALUES ($1, $2, $3, $4, $5, '1.6.0', '1.0', 'active') ON CONFLICT (agent_id) DO UPDATE SET public_key = $4, public_key_fingerprint = $5, status = 'active', revoked_at = NULL;`, agentID, orgID, deviceID, pubKey, fingerprint)

	// Seed User Session in Redis
	tokenHashBytes := sha256.Sum256([]byte(sessionToken))
	tokenHashHex := hex.EncodeToString(tokenHashBytes[:])
	sessionKey := fmt.Sprintf("pcp:session:v1:%s", tokenHashHex)

	sessObj := map[string]interface{}{
		"session_id":      "sess_p16_gate",
		"user_id":         userID,
		"email":           "admin@p16.local",
		"display_name":    "P16 Admin",
		"organization_id": orgID,
		"membership_id":   "mem_p16_01",
		"roles":           []string{"admin"},
		"permissions": map[string]interface{}{
			"*":                  map[string]interface{}{},
			"agent.enroll":       map[string]interface{}{},
			"device.read":        map[string]interface{}{},
			"device.stream.view": map[string]interface{}{},
		},
	}
	sessBytes, _ := json.Marshal(sessObj)
	_ = rdb.Set(ctx, sessionKey, sessBytes, 24*time.Hour).Err()
}

func checkNodeReadiness(nodeURL string, index int) GateResult {
	gate := GateResult{
		Pillar: "1. Production Deployment & Infrastructure",
		Name:   fmt.Sprintf("Node #%d Hardened Readiness Probe (%s)", index+1, nodeURL),
	}

	resp, err := http.Get(fmt.Sprintf("%s/health/ready", nodeURL))
	if err != nil {
		gate.ErrorMessage = fmt.Sprintf("HTTP GET /health/ready failed: %v", err)
		return gate
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		gate.ErrorMessage = fmt.Sprintf("Expected HTTP 200 OK, got HTTP %d", resp.StatusCode)
		return gate
	}

	var res struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		gate.ErrorMessage = fmt.Sprintf("Failed to decode ready response: %v", err)
		return gate
	}

	if res.Status != "up" {
		gate.ErrorMessage = fmt.Sprintf("Expected status 'up', got '%s'", res.Status)
		return gate
	}
	if res.Checks["postgres"] != "up" || res.Checks["redis"] != "up" || res.Checks["outbox_worker"] != "up" || res.Checks["migrations"] != "up" {
		gate.ErrorMessage = fmt.Sprintf("Sub-checks failed: postgres=%s, redis=%s, outbox=%s, migrations=%s", res.Checks["postgres"], res.Checks["redis"], res.Checks["outbox_worker"], res.Checks["migrations"])
		return gate
	}

	gate.Passed = true
	return gate
}

func checkCaddyPerimeter(caddyURL string) GateResult {
	gate := GateResult{
		Pillar: "1. Production Deployment & Caddy Infrastructure",
		Name:   fmt.Sprintf("Caddy Perimeter Reverse Proxy Check (%s)", caddyURL),
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Timeout: 3 * time.Second, Transport: tr}

	resp, err := client.Get(fmt.Sprintf("%s/health/ready", caddyURL))
	if err != nil {
		gate.ErrorMessage = fmt.Sprintf("Caddy perimeter request failed: %v", err)
		return gate
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		gate.ErrorMessage = fmt.Sprintf("Expected HTTP 200 OK from Caddy perimeter, got HTTP %d", resp.StatusCode)
		return gate
	}

	edgeMarker := resp.Header.Get("X-PCP-Edge")
	if edgeMarker != "caddy" {
		gate.ErrorMessage = fmt.Sprintf("Caddy perimeter missing 'X-PCP-Edge: caddy' marker header, got '%s'", edgeMarker)
		return gate
	}

	contentTypeOpt := resp.Header.Get("X-Content-Type-Options")
	if contentTypeOpt != "nosniff" {
		gate.ErrorMessage = fmt.Sprintf("Missing or invalid X-Content-Type-Options security header: '%s'", contentTypeOpt)
		return gate
	}

	gate.Passed = true
	return gate
}

func checkRealAgentAuthHandshake(nodeURL, deviceID, agentID string, privKey ed25519.PrivateKey, fingerprint string) GateResult {
	gate := GateResult{
		Pillar: "2. Real Android Device Fleet Readiness",
		Name:   "Agent Ed25519 WSS Production Canonical Auth & Server Challenge Handshake",
	}

	wsURL := fmt.Sprintf("%s/agent/v1/connect?device_id=%s", strings.Replace(nodeURL, "http", "ws", 1), deviceID)

	timestampStr := strconv.FormatInt(time.Now().Unix(), 10)
	nonceBytes := make([]byte, 16)
	_, _ = rand.Read(nonceBytes)
	nonceStr := hex.EncodeToString(nonceBytes)

	// Canonical String for GET /agent/v1/connect (empty body)
	emptyBodyHash := sha256.Sum256([]byte(""))
	bodyHashHex := hex.EncodeToString(emptyBodyHash[:])
	canonicalPath := "/agent/v1/connect"
	canonicalMsg := fmt.Sprintf("GET\n%s\n%s\n%s\n%s", canonicalPath, bodyHashHex, timestampStr, nonceStr)

	sigBytes := ed25519.Sign(privKey, []byte(canonicalMsg))
	sigB64 := base64.StdEncoding.EncodeToString(sigBytes)

	headers := http.Header{}
	headers.Set("X-Agent-ID", agentID)
	headers.Set("X-Agent-Timestamp", timestampStr)
	headers.Set("X-Agent-Nonce", nonceStr)
	headers.Set("X-Agent-Signature", sigB64)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		gate.ErrorMessage = fmt.Sprintf("Production WSS dial failed: %v", err)
		return gate
	}
	defer conn.Close()

	// Read Server Challenge
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		gate.ErrorMessage = fmt.Sprintf("Failed to read WSS server challenge: %v", err)
		return gate
	}

	var challengeEnv agentws.WSEnvelope
	if err := json.Unmarshal(msg, &challengeEnv); err != nil || challengeEnv.Type != agentws.MessageTypeServerChallenge {
		gate.ErrorMessage = fmt.Sprintf("Invalid server challenge payload: %s", string(msg))
		return gate
	}

	var challengePayload agentws.ServerChallengePayload
	_ = json.Unmarshal(challengeEnv.Payload, &challengePayload)

	// Sign Challenge Nonce
	chalSig := ed25519.Sign(privKey, []byte(challengePayload.ChallengeNonce))
	respPayload := agentws.AgentChallengeResponsePayload{
		ChallengeSignature: base64.StdEncoding.EncodeToString(chalSig),
	}
	respEnv, _ := agentws.NewWSEnvelope(agentws.MessageTypeAgentChallengeResponse, "chal_resp_01", respPayload)
	respBytes, _ := json.Marshal(respEnv)

	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, respBytes); err != nil {
		gate.ErrorMessage = fmt.Sprintf("Failed to send challenge response: %v", err)
		return gate
	}

	// Read Connection Ready
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, readyMsg, err := conn.ReadMessage()
	if err != nil {
		gate.ErrorMessage = fmt.Sprintf("Failed to read connection ready message: %v", err)
		return gate
	}

	var readyEnv agentws.WSEnvelope
	if err := json.Unmarshal(readyMsg, &readyEnv); err != nil || readyEnv.Type != agentws.MessageTypeConnectionReady {
		gate.ErrorMessage = fmt.Sprintf("Expected connection.ready, got: %s", string(readyMsg))
		return gate
	}

	gate.Passed = true
	return gate
}

func checkRealRevocationTeardown(nodeURL_A, nodeURL_B string, pool *pgxpool.Pool, orgID, deviceID, agentID string, privKey ed25519.PrivateKey, fingerprint, sessionToken string) GateResult {
	gate := GateResult{
		Pillar: "2. Real Android Device Fleet Readiness",
		Name:   "Live Agent Credential Revocation Cross-Node WS Teardown & 401 Reconnect Rejection Proof",
	}

	wsURL_A := fmt.Sprintf("%s/agent/v1/connect?device_id=%s", strings.Replace(nodeURL_A, "http", "ws", 1), deviceID)

	timestampStr := strconv.FormatInt(time.Now().Unix(), 10)
	nonceBytes := make([]byte, 16)
	_, _ = rand.Read(nonceBytes)
	nonceStr := hex.EncodeToString(nonceBytes)

	emptyBodyHash := sha256.Sum256([]byte(""))
	bodyHashHex := hex.EncodeToString(emptyBodyHash[:])
	canonicalMsg := fmt.Sprintf("GET\n/agent/v1/connect\n%s\n%s\n%s", bodyHashHex, timestampStr, nonceStr)
	sigB64 := base64.StdEncoding.EncodeToString(ed25519.Sign(privKey, []byte(canonicalMsg)))

	headers := http.Header{}
	headers.Set("X-Agent-ID", agentID)
	headers.Set("X-Agent-Timestamp", timestampStr)
	headers.Set("X-Agent-Nonce", nonceStr)
	headers.Set("X-Agent-Signature", sigB64)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL_A, headers)
	if err != nil {
		gate.ErrorMessage = fmt.Sprintf("Failed to connect Agent to Node A: %v", err)
		return gate
	}
	defer conn.Close()

	// Complete WSS Challenge Handshake on Node A
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, _ := conn.ReadMessage()
	var challengeEnv agentws.WSEnvelope
	_ = json.Unmarshal(msg, &challengeEnv)
	var challengePayload agentws.ServerChallengePayload
	_ = json.Unmarshal(challengeEnv.Payload, &challengePayload)

	chalSig := ed25519.Sign(privKey, []byte(challengePayload.ChallengeNonce))
	respPayload := agentws.AgentChallengeResponsePayload{ChallengeSignature: base64.StdEncoding.EncodeToString(chalSig)}
	respEnv, _ := agentws.NewWSEnvelope(agentws.MessageTypeAgentChallengeResponse, "chal_resp_02", respPayload)
	respBytes, _ := json.Marshal(respEnv)
	_ = conn.WriteMessage(websocket.TextMessage, respBytes)

	// Read connection.ready
	_, _, _ = conn.ReadMessage()

	// Execute Admin Revocation API call on Node B
	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/agents/%s", nodeURL_B, agentID), nil)
	req.Header.Set("Cookie", fmt.Sprintf("__Host-pcp_session=%s", sessionToken))

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		gate.ErrorMessage = fmt.Sprintf("Admin Revocation DELETE request on Node B failed (status=%d, err=%v)", status, err)
		return gate
	}

	// Verify WebSocket on Node A receives disconnect / close code 4001 or socket error within 3 seconds
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, _, readErr := conn.ReadMessage()
	if readErr == nil {
		gate.ErrorMessage = "Expected WebSocket teardown on Node A after admin revocation on Node B, but socket remained open"
		return gate
	}

	// Verify PostgreSQL status = 'revoked'
	var status string
	err = pool.QueryRow(context.Background(), "SELECT status FROM device_agents WHERE agent_id = $1", agentID).Scan(&status)
	if err != nil || status != "revoked" {
		gate.ErrorMessage = fmt.Sprintf("Expected PostgreSQL status='revoked', got status='%s' (err=%v)", status, err)
		return gate
	}

	// Verify Reconnect attempt with SAME valid key is rejected with HTTP 401
	timestampStr2 := strconv.FormatInt(time.Now().Unix(), 10)
	nonceStr2 := "nonce_reconnect_01"
	canonicalMsg2 := fmt.Sprintf("GET\n/agent/v1/connect\n%s\n%s\n%s", bodyHashHex, timestampStr2, nonceStr2)
	sigB64_2 := base64.StdEncoding.EncodeToString(ed25519.Sign(privKey, []byte(canonicalMsg2)))

	headers2 := http.Header{}
	headers2.Set("X-Agent-ID", agentID)
	headers2.Set("X-Agent-Timestamp", timestampStr2)
	headers2.Set("X-Agent-Nonce", nonceStr2)
	headers2.Set("X-Agent-Signature", sigB64_2)

	_, resp2, dialErr := websocket.DefaultDialer.Dial(wsURL_A, headers2)
	if dialErr == nil {
		gate.ErrorMessage = "Revoked Agent key reconnect succeeded, expected HTTP 401 rejection"
		return gate
	}
	if resp2 != nil && resp2.StatusCode != http.StatusUnauthorized {
		gate.ErrorMessage = fmt.Sprintf("Expected HTTP 401 Unauthorized on revoked reconnect, got HTTP %d", resp2.StatusCode)
		return gate
	}

	gate.Passed = true
	return gate
}

func checkRealViewerQuotaLimit(nodeURL string, pool *pgxpool.Pool, rdb *redis.Client, orgID, userID, sessionToken string) GateResult {
	gate := GateResult{
		Pillar: "4. Real Media / WebRTC Production Gate",
		Name:   "Distributed Single Viewer Quota Limit Enforcement (Max 1)",
	}

	devQuotaID := "dev_quota_01"
	agentQuotaID := "agt_quota_01"
	pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)
	fingerprint := crypto.ComputePublicKeyFingerprint(pubKey)

	seedEntities(context.Background(), pool, rdb, orgID, userID, devQuotaID, agentQuotaID, pubKey, fingerprint, sessionToken)

	// Connect Agent for devQuotaID
	wsURL := fmt.Sprintf("%s/agent/v1/connect?device_id=%s", strings.Replace(nodeURL, "http", "ws", 1), devQuotaID)
	timestampStr := strconv.FormatInt(time.Now().Unix(), 10)
	nonceStr := "nonce_quota_01"
	emptyBodyHash := sha256.Sum256([]byte(""))
	bodyHashHex := hex.EncodeToString(emptyBodyHash[:])
	canonicalMsg := fmt.Sprintf("GET\n/agent/v1/connect\n%s\n%s\n%s", bodyHashHex, timestampStr, nonceStr)
	sigB64 := base64.StdEncoding.EncodeToString(ed25519.Sign(privKey, []byte(canonicalMsg)))

	headers := http.Header{}
	headers.Set("X-Agent-ID", agentQuotaID)
	headers.Set("X-Agent-Timestamp", timestampStr)
	headers.Set("X-Agent-Nonce", nonceStr)
	headers.Set("X-Agent-Signature", sigB64)

	agentConn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		gate.ErrorMessage = fmt.Sprintf("Failed to connect Agent for quota test: %v", err)
		return gate
	}
	defer agentConn.Close()

	// Complete Agent Challenge Handshake
	_ = agentConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, _ := agentConn.ReadMessage()
	var challengeEnv agentws.WSEnvelope
	_ = json.Unmarshal(msg, &challengeEnv)
	var challengePayload agentws.ServerChallengePayload
	_ = json.Unmarshal(challengeEnv.Payload, &challengePayload)

	chalSig := ed25519.Sign(privKey, []byte(challengePayload.ChallengeNonce))
	respPayload := agentws.AgentChallengeResponsePayload{ChallengeSignature: base64.StdEncoding.EncodeToString(chalSig)}
	respEnv, _ := agentws.NewWSEnvelope(agentws.MessageTypeAgentChallengeResponse, "chal_resp_q", respPayload)
	respBytes, _ := json.Marshal(respEnv)
	_ = agentConn.WriteMessage(websocket.TextMessage, respBytes)
	_, _, _ = agentConn.ReadMessage() // connection.ready

	// Viewer #1 Connects with authenticated Session Cookie
	mediaWSURL := fmt.Sprintf("%s/api/v1/devices/%s/media/ws", strings.Replace(nodeURL, "http", "ws", 1), devQuotaID)
	vHeaders1 := http.Header{}
	vHeaders1.Set("Cookie", fmt.Sprintf("__Host-pcp_session=%s", sessionToken))

	v1Conn, _, err := websocket.DefaultDialer.Dial(mediaWSURL, vHeaders1)
	if err != nil {
		gate.ErrorMessage = fmt.Sprintf("Viewer #1 media stream dial failed: %v", err)
		return gate
	}

	// Viewer #2 Connects to SAME device -> Expect HTTP 429 Too Many Requests
	vHeaders2 := http.Header{}
	vHeaders2.Set("Cookie", fmt.Sprintf("__Host-pcp_session=%s", sessionToken))

	_, v2Resp, v2Err := websocket.DefaultDialer.Dial(mediaWSURL, vHeaders2)
	if v2Err == nil {
		v1Conn.Close()
		gate.ErrorMessage = "Viewer #2 dial succeeded, expected HTTP 429 quota rejection"
		return gate
	}
	if v2Resp == nil || v2Resp.StatusCode != http.StatusTooManyRequests {
		v1Conn.Close()
		status := 0
		if v2Resp != nil {
			status = v2Resp.StatusCode
		}
		gate.ErrorMessage = fmt.Sprintf("Expected HTTP 429 Too Many Requests for Viewer #2, got HTTP %d", status)
		return gate
	}

	// Close Viewer #1 -> Lease released
	v1Conn.Close()
	time.Sleep(100 * time.Millisecond)

	// Viewer #3 Connects -> Expect Success (101 / Connected)
	vHeaders3 := http.Header{}
	vHeaders3.Set("Cookie", fmt.Sprintf("__Host-pcp_session=%s", sessionToken))

	v3Conn, _, v3Err := websocket.DefaultDialer.Dial(mediaWSURL, vHeaders3)
	if v3Err != nil {
		gate.ErrorMessage = fmt.Sprintf("Viewer #3 dial failed after Viewer #1 released lease: %v", v3Err)
		return gate
	}
	v3Conn.Close()

	gate.Passed = true
	return gate
}

func checkBackupRestoreSmoke(ctx context.Context, pool *pgxpool.Pool, pgURL string) GateResult {
	gate := GateResult{
		Pillar: "1. Production Deployment & Infrastructure",
		Name:   "PostgreSQL Backup, Integrity & Restore Smoke Proof",
	}

	pgDumpPath, err := exec.LookPath("pg_dump")
	if err != nil {
		gate.ErrorMessage = "HARD FAIL: pg_dump CLI utility missing on system PATH"
		return gate
	}

	psqlPath, err := exec.LookPath("psql")
	if err != nil {
		gate.ErrorMessage = "HARD FAIL: psql CLI utility missing on system PATH for restore verification"
		return gate
	}

	testSerial := fmt.Sprintf("SN_BACKUP_TEST_%d", time.Now().UnixNano())
	testDevID := fmt.Sprintf("dev_bkp_%d", time.Now().UnixNano())
	orgID := "org_p16_gate"

	_, err = pool.Exec(ctx, `
		INSERT INTO devices (device_id, organization_id, name, serial_number, model, platform_version, status)
		VALUES ($1, $2, 'Backup Integrity Benchmark Device', $3, 'BkpModel', 'Android 15', 'online')
		ON CONFLICT (organization_id, device_id) DO UPDATE SET serial_number = $3;
	`, testDevID, orgID, testSerial)
	if err != nil {
		gate.ErrorMessage = fmt.Sprintf("Failed to seed backup benchmark record: %v", err)
		return gate
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM devices WHERE device_id = $1", testDevID)
	}()

	var preOrgCnt, preDevCnt int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM organizations").Scan(&preOrgCnt)
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM devices").Scan(&preDevCnt)

	backupFile := "/tmp/pcp_real_data_backup.sql"
	cmd := exec.Command(pgDumpPath, "--dbname="+pgURL, "--file="+backupFile, "--clean", "--if-exists")
	if out, err := cmd.CombinedOutput(); err != nil {
		gate.ErrorMessage = fmt.Sprintf("pg_dump data backup failed: %v (output: %s)", err, string(out))
		return gate
	}
	defer func() {
		_ = os.Remove(backupFile)
	}()

	sqlBytes, err := os.ReadFile(backupFile)
	if err != nil || len(sqlBytes) == 0 {
		gate.ErrorMessage = "Backup file unreadable or zero bytes"
		return gate
	}

	if !strings.Contains(string(sqlBytes), testSerial) {
		gate.ErrorMessage = "Backup file content integrity check failed: seeded record serial missing from SQL dump"
		return gate
	}

	// Create Isolated Verification Database (pcp_restore_verify)
	_, _ = pool.Exec(ctx, "DROP DATABASE IF EXISTS pcp_restore_verify;")
	_, err = pool.Exec(ctx, "CREATE DATABASE pcp_restore_verify TEMPLATE template0;")
	if err != nil {
		gate.ErrorMessage = fmt.Sprintf("Failed to create isolated restore verification DB 'pcp_restore_verify': %v", err)
		return gate
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), "DROP DATABASE IF EXISTS pcp_restore_verify;")
	}()

	verifyPGURL := strings.Replace(pgURL, "/phone_farm", "/pcp_restore_verify", 1)
	verifyPGURL = strings.Replace(verifyPGURL, "/phone_control_platform", "/pcp_restore_verify", 1)

	restoreCmd := exec.Command(psqlPath, verifyPGURL, "-X", "-v", "ON_ERROR_STOP=1", "-f", backupFile)
	if out, err := restoreCmd.CombinedOutput(); err != nil {
		gate.ErrorMessage = fmt.Sprintf("psql restore into pcp_restore_verify failed: %v (output: %s)", err, string(out))
		return gate
	}

	verifyPool, err := pgxpool.New(ctx, verifyPGURL)
	if err != nil {
		gate.ErrorMessage = fmt.Sprintf("Failed to connect to restored database pcp_restore_verify: %v", err)
		return gate
	}
	defer verifyPool.Close()

	var postDevCnt int
	err = verifyPool.QueryRow(ctx, "SELECT COUNT(*) FROM devices WHERE serial_number = $1", testSerial).Scan(&postDevCnt)
	if err != nil || postDevCnt != 1 {
		gate.ErrorMessage = fmt.Sprintf("Post-restore integrity check on pcp_restore_verify failed: expected 1 restored test device, got %d (err: %v)", postDevCnt, err)
		return gate
	}

	var postTotalDevCnt int
	_ = verifyPool.QueryRow(ctx, "SELECT COUNT(*) FROM devices").Scan(&postTotalDevCnt)
	if postTotalDevCnt != preDevCnt {
		gate.ErrorMessage = fmt.Sprintf("Post-restore device count mismatch: source had %d devices, restored had %d", preDevCnt, postTotalDevCnt)
		return gate
	}

	var fkOrphans int
	err = verifyPool.QueryRow(ctx, "SELECT COUNT(*) FROM device_agents a LEFT JOIN devices d ON a.device_id = d.device_id WHERE d.device_id IS NULL").Scan(&fkOrphans)
	if err != nil || fkOrphans > 0 {
		gate.ErrorMessage = fmt.Sprintf("Foreign key integrity check on restored DB failed: %d orphaned agent records found", fkOrphans)
		return gate
	}

	gate.Passed = true
	return gate
}
