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
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)

type LoadTestReport struct {
	Timestamp       string  `json:"timestamp"`
	Duration        string  `json:"duration"`
	TargetNodeURL   string  `json:"target_node_url"`
	ActiveWorkers   int     `json:"active_workers"`
	Submitted       uint64  `json:"submitted"`
	Dispatched      uint64  `json:"dispatched"`
	Acked           uint64  `json:"acked"`
	Succeeded       uint64  `json:"succeeded"`
	Failed          uint64  `json:"failed"`
	Timeouts        uint64  `json:"timeouts"`
	Duplicates      uint64  `json:"duplicates"`
	ErrorRatePct    float64 `json:"error_rate_pct"`
	P50LatencyMs    float64 `json:"p50_latency_ms"`
	P95LatencyMs    float64 `json:"p95_latency_ms"`
	P99LatencyMs    float64 `json:"p99_latency_ms"`
	GoroutinesCount int     `json:"goroutines_count"`
	HeapAllocBytes  uint64  `json:"heap_alloc_bytes"`
	Status          string  `json:"status"`
}

type SyntheticAgentWorker struct {
	AgentID   string
	DeviceID  string
	PubKey    ed25519.PublicKey
	PrivKey   ed25519.PrivateKey
	Conn      *websocket.Conn
	stopChan  chan struct{}
}

func seedSyntheticDevicesAndStartAgents(ctx context.Context, targetNodeURL, dbURL, redisURL, sessionToken string, count int) ([]*SyntheticAgentWorker, func()) {
	if dbURL == "" {
		return nil, func() {}
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("Failed to connect pgxpool in loadharness", "error", err)
		return nil, func() {}
	}
	defer pool.Close()

	orgID := "org_dev_01"
	userID := "user_dev_01"

	// 1. Seed Organization & User
	_, err = pool.Exec(ctx, `
		INSERT INTO organizations (organization_id, name, slug)
		VALUES ($1, 'Dev Org', 'dev-org-01')
		ON CONFLICT (organization_id) DO NOTHING;
	`, orgID)
	if err != nil {
		slog.Error("Failed to seed organization", "error", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO users (user_id, email, password_hash, display_name)
		VALUES ($1, 'synth_user@dev.local', 'hash', 'Synth User')
		ON CONFLICT (user_id) DO NOTHING;
	`, userID)
	if err != nil {
		slog.Error("Failed to seed user", "error", err)
	}

	// 2. Seed Real Redis Browser Session
	if redisURL != "" {
		rURL := redisURL
		if !strings.HasPrefix(rURL, "redis://") && !strings.HasPrefix(rURL, "rediss://") {
			rURL = "redis://" + rURL
		}
		if opt, err := redis.ParseURL(rURL); err == nil {
			rdb := redis.NewClient(opt)
			tokenHashBytes := sha256.Sum256([]byte(sessionToken))
			tokenHashHex := hex.EncodeToString(tokenHashBytes[:])
			sessionKey := fmt.Sprintf("pcp:session:v1:%s", tokenHashHex)

			sessObj := map[string]interface{}{
				"session_id":      "sess_synth_01",
				"user_id":         userID,
				"email":           "synth_user@dev.local",
				"display_name":    "Synth Operator User",
				"organization_id": orgID,
				"membership_id":   "mem_synth_01",
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

	var workers []*SyntheticAgentWorker

	// 3. Seed Devices, Control Leases, Device Agents & Connect WS Agents
	for i := 0; i < count; i++ {
		deviceID := fmt.Sprintf("dev_synth_%02d", i)
		serial := fmt.Sprintf("SN_SYNTH_%02d", i)
		leaseID := fmt.Sprintf("lease_synth_%d", i)
		agentID := fmt.Sprintf("agent_synth_%02d", i)

		pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			slog.Error("Failed to generate Agent Ed25519 key", "error", err)
			continue
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO devices (device_id, organization_id, name, serial_number, model, platform_version, status)
			VALUES ($1, $2, $3, $4, 'SynthModel', 'Android 14', 'online')
			ON CONFLICT (device_id) DO UPDATE SET status = 'online', updated_at = NOW();
		`, deviceID, orgID, deviceID, serial)
		if err != nil {
			slog.Error("Failed to seed device", "device_id", deviceID, "error", err)
		}

		expiresAt := time.Now().Add(1 * time.Hour)
		_, err = pool.Exec(ctx, `
			INSERT INTO control_leases (control_lease_id, organization_id, user_id, device_id, expires_at, fencing_token)
			VALUES ($1, $2, $3, $4, $5, 1)
			ON CONFLICT (control_lease_id) DO UPDATE SET expires_at = $5;
		`, leaseID, orgID, userID, deviceID, expiresAt)
		if err != nil {
			slog.Error("Failed to seed control lease", "lease_id", leaseID, "error", err)
		}

		// Seed Agent entity in database
		fp := crypto.ComputePublicKeyFingerprint(pubKey)
		_, err = pool.Exec(ctx, `
			INSERT INTO device_agents (agent_id, organization_id, device_id, public_key, public_key_fingerprint, apk_version, protocol_version, status, credential_version)
			VALUES ($1, $2, $3, $4, $5, '1.0.0', '1.0', 'active', 1)
			ON CONFLICT (agent_id) DO UPDATE SET public_key = $4, public_key_fingerprint = $5;
		`, agentID, orgID, deviceID, []byte(pubKey), fp)
		if err != nil {
			slog.Error("Failed to seed device agent", "agent_id", agentID, "error", err)
		}

		// Seed Active Redis Lease
		if redisURL != "" {
			rURL := redisURL
			if !strings.HasPrefix(rURL, "redis://") && !strings.HasPrefix(rURL, "rediss://") {
				rURL = "redis://" + rURL
			}
			if opt, err := redis.ParseURL(rURL); err == nil {
				rdb := redis.NewClient(opt)
				now := time.Now().UTC()
				exp := now.Add(1 * time.Hour)
				leaseObj := map[string]interface{}{
					"control_lease_id":  leaseID,
					"device_id":        deviceID,
					"organization_id":  orgID,
					"user_id":          userID,
					"user_display_name": "Synth User",
					"fencing_token":    1,
					"acquired_at":      now.Format(time.RFC3339),
					"expires_at":       exp.Format(time.RFC3339),
					"ttl_seconds":      3600,
				}
				data, _ := json.Marshal(leaseObj)
				leaseKey := fmt.Sprintf("pcp:control:lease:v1:%s:%s", orgID, deviceID)
				_ = rdb.Set(ctx, leaseKey, data, 1*time.Hour).Err()
				rdb.Close()
			}
		}

		// Connect Background Synthetic Agent WS
		wsConn := connectSyntheticAgentWS(ctx, targetNodeURL, deviceID, agentID, privKey)
		if wsConn != nil {
			worker := &SyntheticAgentWorker{
				AgentID:  agentID,
				DeviceID: deviceID,
				PubKey:   pubKey,
				PrivKey:  privKey,
				Conn:     wsConn,
				stopChan: make(chan struct{}),
			}
			workers = append(workers, worker)

			// Start Agent Loop to auto-ACK incoming commands
			go func(w *SyntheticAgentWorker) {
				for {
					select {
					case <-w.stopChan:
						return
					default:
						_ = w.Conn.SetReadDeadline(time.Now().Add(1 * time.Second))
						_, msg, err := w.Conn.ReadMessage()
						if err != nil {
							continue
						}
						var env struct {
							Type    string                 `json:"type"`
							Payload map[string]interface{} `json:"payload"`
						}
						if json.Unmarshal(msg, &env) == nil && (env.Type == "command" || env.Type == "device.command") {
							cmdID, _ := env.Payload["commandId"].(string)
							if cmdID != "" {
								// Send ACK
								ackPayload := map[string]interface{}{
									"type": "command.status",
									"payload": map[string]interface{}{
										"commandId": cmdID,
										"status":    "ack",
										"sequence":  1,
									},
								}
								ackBytes, _ := json.Marshal(ackPayload)
								_ = w.Conn.WriteMessage(websocket.TextMessage, ackBytes)

								// Send Succeeded
								succPayload := map[string]interface{}{
									"type": "command.status",
									"payload": map[string]interface{}{
										"commandId": cmdID,
										"status":    "succeeded",
										"sequence":  2,
									},
								}
								succBytes, _ := json.Marshal(succPayload)
								_ = w.Conn.WriteMessage(websocket.TextMessage, succBytes)
							}
						}
					}
				}
			}(worker)
		}
	}

	cleanup := func() {
		for _, w := range workers {
			close(w.stopChan)
			if w.Conn != nil {
				_ = w.Conn.Close()
			}
		}
	}

	return workers, cleanup
}

func connectSyntheticAgentWS(ctx context.Context, nodeURL, deviceID, agentID string, privKey ed25519.PrivateKey) *websocket.Conn {
	wsURL := strings.Replace(nodeURL, "http://", "ws://", 1) + "/agent/v1/connect?device_id=" + deviceID

	timestampStr := strconv.FormatInt(time.Now().Unix(), 10)
	nonceBytes := make([]byte, 16)
	_, _ = rand.Read(nonceBytes)
	nonce := hex.EncodeToString(nonceBytes)

	emptyBodyHash := sha256.Sum256([]byte(""))
	bodyHashHex := hex.EncodeToString(emptyBodyHash[:])
	canonicalMsg := fmt.Sprintf("%s\n%s\n%s\n%s\n%s", "GET", "/agent/v1/connect", bodyHashHex, timestampStr, nonce)
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
		slog.Warn("Failed to connect synthetic agent WS", "device_id", deviceID, "error", err)
		return nil
	}

	// Complete WSS Challenge Handshake
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		return nil
	}

	var challengeEnv struct {
		Type    string `json:"type"`
		Payload struct {
			ChallengeNonce string `json:"challenge_nonce"`
		} `json:"payload"`
	}
	if json.Unmarshal(msg, &challengeEnv) != nil {
		_ = conn.Close()
		return nil
	}

	nonceBytes, _ = base64.StdEncoding.DecodeString(challengeEnv.Payload.ChallengeNonce)
	sig := ed25519.Sign(privKey, nonceBytes)
	sigB64 = base64.StdEncoding.EncodeToString(sig)

	respPayload := map[string]interface{}{
		"type": "agent_challenge_response",
		"payload": map[string]interface{}{
			"challenge_signature": sigB64,
		},
	}
	respBytes, _ := json.Marshal(respPayload)
	_ = conn.WriteMessage(websocket.TextMessage, respBytes)

	// Read connection.ready confirmation
	_, _, _ = conn.ReadMessage()
	_ = conn.SetReadDeadline(time.Time{})
	return conn
}

func main() {
	durationFlag := flag.Duration("duration", 5*time.Second, "Load test run duration")
	concurrencyFlag := flag.Int("concurrency", 10, "Number of concurrent synthetic device workers")
	rateFlag := flag.Int("rate-per-min", 300, "Target total commands per minute")
	nodeURLFlag := flag.String("target-url", "http://localhost:8083", "Target Go Backend Server URL")
	flag.Parse()

	slog.Info("Starting Phase 1.5 Real Platform Command Load Harness",
		"duration", *durationFlag,
		"concurrency", *concurrencyFlag,
		"target_rate_per_min", *rateFlag,
		"target_url", *nodeURLFlag,
	)

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("POSTGRES_URL")
	}
	redisURL := os.Getenv("REDIS_URL")
	sessionToken := "harness_session_token_synth"

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var dbPool *pgxpool.Pool
	if dbURL != "" {
		if pool, err := pgxpool.New(ctx, dbURL); err == nil {
			if pool.Ping(ctx) == nil {
				dbPool = pool
			}
		}
	}

	if dbPool == nil {
		slog.Error("FATAL: Database connection unavailable for loadharness")
		os.Exit(1)
	}

	// Seed synthetic devices & launch connected Agent WS clients
	_, cleanupAgents := seedSyntheticDevicesAndStartAgents(ctx, *nodeURLFlag, dbURL, redisURL, sessionToken, *concurrencyFlag)
	defer cleanupAgents()

	var submitted atomic.Uint64
	var dispatched atomic.Uint64
	var acked atomic.Uint64
	var succeeded atomic.Uint64
	var failed atomic.Uint64
	var timeouts atomic.Uint64
	var duplicates atomic.Uint64

	var latencies []float64
	var latMu sync.Mutex

	stopChan := make(chan struct{})
	timer := time.AfterFunc(*durationFlag, func() {
		close(stopChan)
	})
	defer timer.Stop()

	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}

	var trackerWg sync.WaitGroup
	var wg sync.WaitGroup
	intervalPerWorker := time.Duration(float64(time.Minute) / (float64(*rateFlag) / float64(*concurrencyFlag)))
	if intervalPerWorker <= 0 {
		intervalPerWorker = 50 * time.Millisecond
	}

	for i := 0; i < *concurrencyFlag; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			ticker := time.NewTicker(intervalPerWorker)
			defer ticker.Stop()

			deviceID := fmt.Sprintf("dev_synth_%02d", workerID)

			doWork := func() {
				start := time.Now()
				submitted.Add(1)

				payload := map[string]interface{}{
					"deviceId":       deviceID,
					"type":           "gesture.touch",
					"controlLeaseId": fmt.Sprintf("lease_synth_%d", workerID),
					"idempotencyKey": fmt.Sprintf("idem_synth_%d_%d", workerID, time.Now().UnixNano()),
					"payload": map[string]interface{}{
						"coordinateSpace": "normalized_display_v1",
						"orientation":     "portrait",
						"x":               0.5,
						"y":               0.5,
					},
				}
				bodyBytes, _ := json.Marshal(payload)

				reqURL := fmt.Sprintf("%s/api/v1/commands", *nodeURLFlag)
				req, err := http.NewRequestWithContext(context.Background(), "POST", reqURL, bytes.NewReader(bodyBytes))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Cookie", fmt.Sprintf("__Host-pcp_session=%s", sessionToken))
				req.Header.Set("Origin", "http://localhost:3000")

				var ok bool
				if err == nil {
					resp, httpErr := httpClient.Do(req)
					if httpErr == nil {
						if resp.StatusCode == http.StatusAccepted {
							ok = true
							dispatched.Add(1)
							var resBody struct {
								Data struct {
									CommandID string `json:"commandId"`
								} `json:"data"`
							}
							if json.NewDecoder(resp.Body).Decode(&resBody) == nil && resBody.Data.CommandID != "" {
								cmdID := resBody.Data.CommandID
								trackerWg.Add(1)
								go func(cID string) {
									defer trackerWg.Done()
									// Poll DB status post-202 for authoritative ACK / Succeeded
									tCtx, tCancel := context.WithTimeout(context.Background(), 5*time.Second)
									defer tCancel()

									tTicker := time.NewTicker(50 * time.Millisecond)
									defer tTicker.Stop()

									var gotAck, gotSucc bool
									for {
										select {
										case <-tCtx.Done():
											if !gotSucc {
												timeouts.Add(1)
											}
											return
										case <-tTicker.C:
											var status string
											err := dbPool.QueryRow(tCtx, "SELECT status FROM commands WHERE command_id = $1", cID).Scan(&status)
											if err == nil {
												if !gotAck && (status == "ack" || status == "executing" || status == "succeeded") {
													acked.Add(1)
													gotAck = true
												}
												if !gotSucc && status == "succeeded" {
													succeeded.Add(1)
													gotSucc = true
													return
												}
											}
										}
									}
								}(cmdID)
							}
						} else if resp.StatusCode == http.StatusTooManyRequests {
							duplicates.Add(1)
						} else {
							failed.Add(1)
							if workerID == 0 {
								buf := new(bytes.Buffer)
								_, _ = buf.ReadFrom(resp.Body)
								slog.Error("Loadharness request rejected", "status_code", resp.StatusCode, "body", buf.String())
							}
						}
						_ = resp.Body.Close()
					} else {
						failed.Add(1)
					}
				} else {
					failed.Add(1)
				}

				latency := float64(time.Since(start).Microseconds()) / 1000.0
				if ok {
					latMu.Lock()
					latencies = append(latencies, latency)
					latMu.Unlock()
				}
			}

			// Perform initial work immediately
			doWork()

			for {
				select {
				case <-stopChan:
					return
				case <-ticker.C:
					doWork()
				}
			}
		}(i)
	}

	wg.Wait()
	trackerWg.Wait()
	if dbPool != nil {
		dbPool.Close()
	}

	// Calculate Report Metrics
	totalSubmitted := submitted.Load()
	totalDispatched := dispatched.Load()
	totalAcked := acked.Load()
	totalSucceeded := succeeded.Load()
	totalFailed := failed.Load()
	totalTimeouts := timeouts.Load()
	totalDuplicates := duplicates.Load()

	errorRate := 0.0
	if totalSubmitted > 0 {
		errorRate = float64(totalFailed) / float64(totalSubmitted) * 100.0
	}

	var p50, p95, p99 float64
	latMu.Lock()
	if len(latencies) > 0 {
		sort.Float64s(latencies)
		p50 = latencies[int(float64(len(latencies))*0.50)]
		p95 = latencies[int(float64(len(latencies))*0.95)]
		p99 = latencies[int(float64(len(latencies))*0.99)]
	}
	latMu.Unlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	statusStr := "PASSED"
	if totalDispatched == 0 || totalAcked < totalDispatched || totalSucceeded < totalDispatched || totalFailed > 0 || totalTimeouts > 0 {
		statusStr = "FAILED"
	}

	report := LoadTestReport{
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		Duration:        durationFlag.String(),
		TargetNodeURL:   *nodeURLFlag,
		ActiveWorkers:   *concurrencyFlag,
		Submitted:       totalSubmitted,
		Dispatched:      totalDispatched,
		Acked:           totalAcked,
		Succeeded:       totalSucceeded,
		Failed:          totalFailed,
		Timeouts:        totalTimeouts,
		Duplicates:      totalDuplicates,
		ErrorRatePct:    errorRate,
		P50LatencyMs:    p50,
		P95LatencyMs:    p95,
		P99LatencyMs:    p99,
		GoroutinesCount: runtime.NumGoroutine(),
		HeapAllocBytes:  m.HeapAlloc,
		Status:          statusStr,
	}

	reportBytes, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(reportBytes))

	if statusStr == "FAILED" {
		os.Exit(1)
	}
}
