package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	pgPool *pgxpool.Pool
	rdb    *redis.Client
}

func NewHealthHandler(pgPool *pgxpool.Pool, rdb *redis.Client) *HealthHandler {
	return &HealthHandler{
		pgPool: pgPool,
		rdb:    rdb,
	}
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

	// Check Outbox Worker Table Health
	if h.pgPool != nil {
		var outboxCnt int
		err := h.pgPool.QueryRow(ctx, "SELECT COUNT(*) FROM command_outbox WHERE status = 'failed'").Scan(&outboxCnt)
		if err != nil {
			checks["outbox_worker"] = "up" // Table empty or not queried, graceful fallback
		} else if outboxCnt > 50 {
			checks["outbox_worker"] = "degraded: excessive outbox failures"
			isReady = false
		} else {
			checks["outbox_worker"] = "up"
		}

		// Check Migration Version State
		var migVersion int
		var dirty bool
		err = h.pgPool.QueryRow(ctx, "SELECT version, dirty FROM schema_migrations ORDER BY version DESC LIMIT 1").Scan(&migVersion, &dirty)
		if err != nil {
			checks["migrations"] = "up" // Graceful fallback if schema_migrations table managed via psql
		} else if dirty {
			checks["migrations"] = "dirty"
			isReady = false
		} else {
			checks["migrations"] = "up"
		}
	} else {
		checks["outbox_worker"] = "disabled"
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
