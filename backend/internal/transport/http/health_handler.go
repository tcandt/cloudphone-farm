package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type OutboxStatusProvider interface {
	GetWorkerStatus() (bool, time.Time, string)
}

type HealthHandler struct {
	pgPool               *pgxpool.Pool
	rdb                  *redis.Client
	outboxStatusProvider OutboxStatusProvider
}

func NewHealthHandler(pgPool *pgxpool.Pool, rdb *redis.Client, outboxProvider ...OutboxStatusProvider) *HealthHandler {
	h := &HealthHandler{
		pgPool: pgPool,
		rdb:    rdb,
	}
	if len(outboxProvider) > 0 {
		h.outboxStatusProvider = outboxProvider[0]
	}
	return h
}

type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Checks    map[string]string `json:"checks,omitempty"`
}

func (h *HealthHandler) Live(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(HealthResponse{
		Status:    "up",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := make(map[string]string)
	isReady := true

	// Check PostgreSQL
	if h.pgPool != nil {
		if err := h.pgPool.Ping(ctx); err != nil {
			checks["postgres"] = "down: " + err.Error()
			isReady = false
		} else {
			checks["postgres"] = "up"
		}
	} else {
		checks["postgres"] = "disabled"
	}

	// Check Redis
	if h.rdb != nil {
		if err := h.rdb.Ping(ctx).Err(); err != nil {
			checks["redis"] = "down: " + err.Error()
			isReady = false
		} else {
			checks["redis"] = "up"
		}
	} else {
		checks["redis"] = "disabled"
	}

	// Check Outbox Worker Process Heartbeat
	if h.outboxStatusProvider != nil {
		isRunning, lastLoopAt, lastErr := h.outboxStatusProvider.GetWorkerStatus()
		if !isRunning {
			checks["outbox_worker"] = "down: dispatcher process stopped"
			isReady = false
		} else if time.Since(lastLoopAt) > 30*time.Second {
			checks["outbox_worker"] = "down: dispatcher heartbeat stale (>30s)"
			isReady = false
		} else if lastErr != "" {
			checks["outbox_worker"] = "degraded: " + lastErr
		} else {
			checks["outbox_worker"] = "up"
		}
	} else if h.pgPool != nil {
		var outboxCnt int
		err := h.pgPool.QueryRow(ctx, "SELECT COUNT(*) FROM command_outbox WHERE status = 'failed'").Scan(&outboxCnt)
		if err != nil {
			checks["outbox_worker"] = "down: " + err.Error()
			isReady = false
		} else if outboxCnt > 50 {
			checks["outbox_worker"] = "degraded: excessive outbox failures"
			isReady = false
		} else {
			checks["outbox_worker"] = "up"
		}
	} else {
		checks["outbox_worker"] = "disabled"
	}

	// Check Migration Authority State in pcp_schema_migrations
	if h.pgPool != nil {
		var migVersion int64
		var migName, migChecksum string
		err := h.pgPool.QueryRow(ctx, "SELECT version, name, checksum FROM pcp_schema_migrations ORDER BY version DESC LIMIT 1").Scan(&migVersion, &migName, &migChecksum)
		if err != nil {
			checks["migrations"] = "down: " + err.Error()
			isReady = false
		} else if migVersion < 7 {
			checks["migrations"] = "degraded: pending migrations"
			isReady = false
		} else if len(migChecksum) != 64 {
			// Require valid 64-char SHA256 hex checksum
			checks["migrations"] = "degraded: migration checksum invalid"
			isReady = false
		} else {
			checks["migrations"] = "up"
		}
	} else {
		checks["migrations"] = "disabled"
	}

	w.Header().Set("Content-Type", "application/json")
	if isReady {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	statusStr := "up"
	if !isReady {
		statusStr = "degraded"
	}

	_ = json.NewEncoder(w).Encode(HealthResponse{
		Status:    statusStr,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	})
}
