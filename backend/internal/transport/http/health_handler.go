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
