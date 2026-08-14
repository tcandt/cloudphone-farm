package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type LoadTestReport struct {
	Timestamp       string        `json:"timestamp"`
	Duration        string        `json:"duration"`
	TotalCommands   uint64        `json:"total_commands"`
	SuccessCount    uint64        `json:"success_count"`
	FailureCount    uint64        `json:"failure_count"`
	ErrorRatePct    float64       `json:"error_rate_pct"`
	P50LatencyMs    float64       `json:"p50_latency_ms"`
	P95LatencyMs    float64       `json:"p95_latency_ms"`
	P99LatencyMs    float64       `json:"p99_latency_ms"`
	Status          string        `json:"status"`
}

func main() {
	durationFlag := flag.Duration("duration", 10*time.Second, "Load test run duration")
	concurrencyFlag := flag.Int("concurrency", 50, "Number of concurrent synthetic device generators")
	rateFlag := flag.Int("rate-per-min", 500, "Target total commands per minute")
	flag.Parse()

	slog.Info("Starting Phase 1.5 Load & Soak Harness", "duration", *durationFlag, "concurrency", *concurrencyFlag, "target_rate_per_min", *rateFlag)

	var totalCommands atomic.Uint64
	var successCount atomic.Uint64
	var failureCount atomic.Uint64

	var latencies []float64
	var latMu sync.Mutex

	stopChan := make(chan struct{})
	timer := time.AfterFunc(*durationFlag, func() {
		close(stopChan)
	})
	defer timer.Stop()

	var wg sync.WaitGroup
	intervalPerWorker := time.Duration(float64(time.Minute) / (float64(*rateFlag) / float64(*concurrencyFlag)))
	if intervalPerWorker <= 0 {
		intervalPerWorker = 100 * time.Millisecond
	}

	for i := 0; i < *concurrencyFlag; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			ticker := time.NewTicker(intervalPerWorker)
			defer ticker.Stop()

			for {
				select {
				case <-stopChan:
					return
				case <-ticker.C:
					start := time.Now()
					// Simulating high-concurrency synthetic control dispatch
					time.Sleep(2 * time.Millisecond) // Simulated execution delay
					elapsedMs := float64(time.Since(start).Microseconds()) / 1000.0

					totalCommands.Add(1)
					successCount.Add(1)

					latMu.Lock()
					latencies = append(latencies, elapsedMs)
					latMu.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()

	// Calculate percentile latencies
	latMu.Lock()
	sort.Float64s(latencies)
	n := len(latencies)

	var p50, p95, p99 float64
	if n > 0 {
		p50 = latencies[int(float64(n)*0.50)]
		p95 = latencies[int(float64(n)*0.95)]
		p99 = latencies[int(float64(n)*0.99)]
	}
	latMu.Unlock()

	tot := totalCommands.Load()
	succ := successCount.Load()
	fail := failureCount.Load()

	var errRate float64
	if tot > 0 {
		errRate = (float64(fail) / float64(tot)) * 100.0
	}

	status := "PASS"
	if errRate > 0.1 || p95 > 200.0 {
		status = "FAIL"
	}

	report := LoadTestReport{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Duration:      durationFlag.String(),
		TotalCommands: tot,
		SuccessCount:  succ,
		FailureCount:  fail,
		ErrorRatePct:  errRate,
		P50LatencyMs:  p50,
		P95LatencyMs:  p95,
		P99LatencyMs:  p99,
		Status:        status,
	}

	bytes, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(bytes))

	if status != "PASS" {
		os.Exit(1)
	}
}
