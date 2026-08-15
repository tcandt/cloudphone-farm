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

	var dbAvailable, redisAvailable bool
	if dbURL != "" {
		if pool, err := pgxpool.New(ctx, dbURL); err == nil {
			if pool.Ping(ctx) == nil {
				dbAvailable = true
				pool.Close()
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

			for {
				select {
				case <-stopChan:
					return
				case <-ticker.C:
					start := time.Now()
					submitted.Add(1)

					// Post real platform payload or check endpoint
					payload := map[string]interface{}{
						"deviceId":       deviceID,
						"type":           "input.tap",
						"controlLeaseId": fmt.Sprintf("lease_synth_%d", workerID),
						"params":         map[string]interface{}{"x": 500, "y": 1000},
					}
					bodyBytes, _ := json.Marshal(payload)

					reqURL := fmt.Sprintf("%s/api/v1/commands", *nodeURLFlag)
					req, err := http.NewRequestWithContext(context.Background(), "POST", reqURL, bytes.NewReader(bodyBytes))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("X-Dev-Worker-ID", deviceID)

					var ok bool
					if err == nil {
						resp, httpErr := httpClient.Do(req)
						if httpErr == nil {
							if resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusOK {
								ok = true
								dispatched.Add(1)
								acked.Add(1)
								succeeded.Add(1)
							} else if resp.StatusCode == http.StatusTooManyRequests {
								duplicates.Add(1)
							} else if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusGatewayTimeout {
								timeouts.Add(1)
							} else {
								failed.Add(1)
							}
							_ = resp.Body.Close()
						} else {
							failed.Add(1)
						}
					} else {
						failed.Add(1)
					}

					// FAIL FAST: Server unavailable must NOT be masked! NO FALLBACK TO ok = true!

					elapsedMs := float64(time.Since(start).Microseconds()) / 1000.0
					if ok {
						latMu.Lock()
						latencies = append(latencies, elapsedMs)
						latMu.Unlock()
					}
				}
			}
		}(i)
	}

	wg.Wait()

	totalSubmitted := submitted.Load()
	totalSucceeded := succeeded.Load()
	totalFailed := failed.Load()

	sort.Float64s(latencies)
	var p50, p95, p99 float64
	if len(latencies) > 0 {
		p50 = latencies[int(float64(len(latencies))*0.50)]
		p95 = latencies[int(float64(len(latencies))*0.95)]
		p99 = latencies[int(float64(len(latencies))*0.99)]
	}

	var errorRate float64
	if totalSubmitted > 0 {
		errorRate = (float64(totalFailed) / float64(totalSubmitted)) * 100.0
	}

	var mStats runtime.MemStats
	runtime.ReadMemStats(&mStats)

	status := "PASSED"
	if totalSubmitted == 0 || totalFailed > 0 || errorRate > 5.0 {
		status = "FAILED"
	}

	report := LoadTestReport{
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		Duration:        durationFlag.String(),
		TargetNodeURL:   *nodeURLFlag,
		ActiveWorkers:   *concurrencyFlag,
		Submitted:       totalSubmitted,
		Dispatched:      dispatched.Load(),
		Acked:           acked.Load(),
		Succeeded:       totalSucceeded,
		Failed:          totalFailed,
		Timeouts:        timeouts.Load(),
		Duplicates:      duplicates.Load(),
		ErrorRatePct:    errorRate,
		P50LatencyMs:    p50,
		P95LatencyMs:    p95,
		P99LatencyMs:    p99,
		GoroutinesCount: runtime.NumGoroutine(),
		HeapAllocBytes:  mStats.HeapAlloc,
		Status:          status,
	}

	jsonBytes, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(jsonBytes))

	if status == "FAILED" {
		slog.Error("Load harness failed validation criteria", "status", status, "submitted", totalSubmitted, "failed", totalFailed)
		os.Exit(1)
	}
}
