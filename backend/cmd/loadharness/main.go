package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
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

func seedSyntheticDevices(ctx context.Context, dbURL, redisURL string, count int) {
	if dbURL == "" {
		return
	}
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("Failed to connect pgxpool in loadharness", "error", err)
		return
	}
	defer pool.Close()

	orgID := "org_dev_01"
	userID := "user_dev_01"

	// 1. Seed Organization & User
	_, err = pool.Exec(ctx, `
		INSERT INTO organizations (organization_id, name, slug, created_at, updated_at)
		VALUES ($1, 'Dev Org', 'dev-org-01', NOW(), NOW())
		ON CONFLICT (organization_id) DO NOTHING;
	`, orgID)
	if err != nil {
		slog.Error("Failed to seed organization", "error", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO users (user_id, email, password_hash, display_name, created_at, updated_at)
		VALUES ($1, 'synth_user@dev.local', 'hash', 'Synth User', NOW(), NOW())
		ON CONFLICT (user_id) DO NOTHING;
	`, userID)
	if err != nil {
		slog.Error("Failed to seed user", "error", err)
	}

	// 2. Seed Devices & Control Leases
	for i := 0; i < count; i++ {
		deviceID := fmt.Sprintf("dev_synth_%02d", i)
		serial := fmt.Sprintf("SN_SYNTH_%02d", i)
		leaseID := fmt.Sprintf("lease_synth_%d", i)

		_, err = pool.Exec(ctx, `
			INSERT INTO devices (device_id, organization_id, name, serial_number, model, platform_version, status, capabilities, created_at, updated_at)
			VALUES ($1, $2, $3, $4, 'SynthModel', 'Android 14', 'online', '{}'::jsonb, NOW(), NOW())
			ON CONFLICT (device_id) DO UPDATE SET status = 'online', updated_at = NOW();
		`, deviceID, orgID, deviceID, serial)
		if err != nil {
			slog.Error("Failed to seed device", "device_id", deviceID, "error", err)
		}

		expiresAt := time.Now().Add(1 * time.Hour)
		_, err = pool.Exec(ctx, `
			INSERT INTO control_leases (control_lease_id, organization_id, user_id, device_id, expires_at, fencing_token, acquired_at)
			VALUES ($1, $2, $3, $4, $5, 1, NOW())
			ON CONFLICT (control_lease_id) DO UPDATE SET expires_at = $5;
		`, leaseID, orgID, userID, deviceID, expiresAt)
		if err != nil {
			slog.Error("Failed to seed control lease", "lease_id", leaseID, "error", err)
		}
	}

	// Active control lease in Redis
	if redisURL != "" {
		if !strings.HasPrefix(redisURL, "redis://") && !strings.HasPrefix(redisURL, "rediss://") {
			redisURL = "redis://" + redisURL
		}
		if opt, err := redis.ParseURL(redisURL); err == nil {
			rdb := redis.NewClient(opt)
			now := time.Now().UTC()
			exp := now.Add(1 * time.Hour)
			for i := 0; i < count; i++ {
				deviceID := fmt.Sprintf("dev_synth_%02d", i)
				leaseID := fmt.Sprintf("lease_synth_%d", i)
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
			}
			rdb.Close()
		}
	}
}

func main() {
	durationFlag := flag.Duration("duration", 5*time.Second, "Load test run duration")
	concurrencyFlag := flag.Int("concurrency", 50, "Number of concurrent synthetic device workers")
	rateFlag := flag.Int("rate-per-min", 500, "Target total commands per minute")
	nodeURLFlag := flag.String("target-url", "http://localhost:8080", "Target Go Backend Server URL")
	flag.Parse()

	slog.Info("Starting Phase 1.5 Real Platform Command Load Harness",
		"duration", *durationFlag,
		"concurrency", *concurrencyFlag,
		"target_rate_per_min", *rateFlag,
		"target_url", *nodeURLFlag,
	)

	// Environment Connectivity Validation
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("POSTGRES_URL")
	}
	redisURL := os.Getenv("REDIS_URL")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var dbPool *pgxpool.Pool
	var dbAvailable, redisAvailable bool
	if dbURL != "" {
		if pool, err := pgxpool.New(ctx, dbURL); err == nil {
			if pool.Ping(ctx) == nil {
				dbAvailable = true
				dbPool = pool
			}
		}
	}

	if redisURL != "" {
		if !strings.HasPrefix(redisURL, "redis://") && !strings.HasPrefix(redisURL, "rediss://") {
			redisURL = "redis://" + redisURL
		}
		if opt, err := redis.ParseURL(redisURL); err == nil {
			rdb := redis.NewClient(opt)
			if rdb.Ping(ctx).Err() == nil {
				redisAvailable = true
			}
			rdb.Close()
		}
	}

	slog.Info("Platform connectivity state", "postgres_live", dbAvailable, "redis_live", redisAvailable)

	if dbAvailable {
		seedSyntheticDevices(ctx, dbURL, redisURL, *concurrencyFlag)
	}

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
				req.Header.Set("X-Dev-User-ID", "user_dev_01")
				req.Header.Set("X-Dev-Org-ID", "org_dev_01")

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
							if json.NewDecoder(resp.Body).Decode(&resBody) == nil && resBody.Data.CommandID != "" && dbPool != nil {
								cmdID := resBody.Data.CommandID
								trackerWg.Add(1)
								go func(cID string) {
									defer trackerWg.Done()
									// Poll DB status post-202 for authoritative ACK / Succeeded
									tCtx, tCancel := context.WithTimeout(context.Background(), 4*time.Second)
									defer tCancel()

									tTicker := time.NewTicker(50 * time.Millisecond)
									defer tTicker.Stop()

									var gotAck, gotSucc bool
									for {
										select {
										case <-tCtx.Done():
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
	if errorRate > 5.0 || totalDispatched == 0 || totalAcked == 0 || totalSucceeded == 0 {
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
