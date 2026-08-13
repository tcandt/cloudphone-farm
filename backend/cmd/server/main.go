package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/internal/config"
	httptransport "github.com/tcandt/cloudphone-farm/backend/internal/transport/http"
)

func main() {
	// Initialize structured logger (log/slog)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.LoadConfig()
	slog.Info("Starting Phone Control Platform Go Backend", "env", cfg.AppEnv, "port", cfg.Port)

	// Context for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize PostgreSQL pool (non-blocking if DB is temporarily unreachable)
	var pgPool *pgxpool.Pool
	pgCtx, pgCancel := context.WithTimeout(ctx, 3*time.Second)
	pool, err := pgxpool.New(pgCtx, cfg.PostgresURL)
	pgCancel()
	if err != nil {
		slog.Warn("Failed to initialize PostgreSQL pool connection", "error", err)
	} else {
		pgPool = pool
		defer pgPool.Close()
		slog.Info("PostgreSQL connection pool initialized")
	}

	// Initialize Redis client
	var rdb *redis.Client
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Warn("Failed to parse Redis URL", "error", err)
	} else {
		rdb = redis.NewClient(opt)
		defer rdb.Close()
		slog.Info("Redis client initialized")
	}

	// Create Chi router
	r := chi.NewRouter()

	// Global Middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CorsAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Request-ID"},
		ExposedHeaders:   []string{"Link", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health Check Handlers
	healthHandler := httptransport.NewHealthHandler(pgPool, rdb)
	r.Get("/health/live", healthHandler.Live)
	r.Get("/health/ready", healthHandler.Ready)

	// API Gateway routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"message":"pong","status":"active"}`))
		})
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Server runner goroutine
	serverErrors := make(chan error, 1)
	go func() {
		slog.Info("HTTP Server listening", "addr", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	// Graceful shutdown listener
	select {
	case err := <-serverErrors:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP Server fatal error", "error", err)
		}
	case <-ctx.Done():
		slog.Info("Shutting down HTTP Server gracefully...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP Server forced shutdown", "error", err)
			_ = server.Close()
		} else {
			slog.Info("HTTP Server stopped cleanly")
		}
	}
}
