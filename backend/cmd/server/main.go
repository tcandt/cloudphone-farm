package main

import (
	"context"
	"encoding/json"
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
	agentservice "github.com/tcandt/cloudphone-farm/backend/internal/agent"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	"github.com/tcandt/cloudphone-farm/backend/internal/command"
	"github.com/tcandt/cloudphone-farm/backend/internal/config"
	devservice "github.com/tcandt/cloudphone-farm/backend/internal/device"
	pgrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
	redisrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
	httptransport "github.com/tcandt/cloudphone-farm/backend/internal/transport/http"
	custommw "github.com/tcandt/cloudphone-farm/backend/internal/transport/http/middleware"
	wstransport "github.com/tcandt/cloudphone-farm/backend/internal/transport/ws"
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

	// Initialize PostgreSQL pool
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

	// Repositories & Services
	userRepo := pgrepo.NewUserRepository(pgPool)
	sessionRepo := redisrepo.NewSessionRepository(rdb)
	deviceRepo := pgrepo.NewDeviceRepository(pgPool, cfg.DeviceOnlineThresholdSeconds, cfg.DeviceOfflineThresholdSeconds)
	enrollRepo := pgrepo.NewEnrollmentRepository(pgPool)
	presenceRepo := redisrepo.NewPresenceRepository(rdb, 30*time.Second)
	outboxRepo := pgrepo.NewOutboxRepository(pgPool)
	cmdRepo := pgrepo.NewCommandRepository(pgPool)
	fenceRepo := pgrepo.NewFenceRepository(pgPool)
	leaseRepo := redisrepo.NewLeaseRepository(rdb)

	// Agent WebSocket Hub & Command Outbox Dispatcher
	wsHub := agentws.NewHub()
	outboxDispatcher := command.NewOutboxDispatcher(outboxRepo, cmdRepo, wsHub)
	outboxDispatcher.Start(ctx)
	defer outboxDispatcher.Stop()

	authService := auth.NewAuthService(userRepo, sessionRepo, time.Duration(cfg.SessionTTLSeconds)*time.Second)
	deviceService := devservice.NewDeviceService(deviceRepo)
	agentService := agentservice.NewAgentService(enrollRepo, presenceRepo, rdb)
	leaseService := devservice.NewLeaseService(fenceRepo, leaseRepo)
	cmdService := command.NewCommandService(pgPool, leaseService)

	// Handlers & Middlewares
	healthHandler := httptransport.NewHealthHandler(pgPool, rdb)
	authHandler := httptransport.NewAuthHandler(authService, cfg)
	deviceHandler := httptransport.NewDeviceHandler(deviceService)
	agentHandler := httptransport.NewAgentHandler(agentService, rdb)
	agentWSHandler := wstransport.NewAgentWSHandler(wsHub, enrollRepo, cmdRepo)
	browserMediaHandler := wstransport.NewBrowserMediaHandler(wsHub, deviceService, cfg.CorsAllowedOrigins)
	leaseHandler := httptransport.NewLeaseHandler(leaseService)
	commandHandler := httptransport.NewCommandHandler(cmdService)

	authMiddleware := custommw.NewAuthMiddleware(authService, cfg.SessionCookieName)
	agentAuthMiddleware := custommw.NewAgentAuthMiddleware(enrollRepo, rdb)

	// Create Chi router
	r := chi.NewRouter()

	// Global Middlewares (No HTTP Timeout on Root Router for Persistent WebSockets)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(custommw.SecurityHeadersMiddleware)
	r.Use(custommw.CSRFMiddleware(cfg.CorsAllowedOrigins))

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CorsAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Request-ID", "X-Agent-Fingerprint", "X-Agent-ID", "X-Agent-Timestamp", "X-Agent-Nonce", "X-Agent-Signature"},
		ExposedHeaders:   []string{"Link", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health Check Handlers (Public)
	r.Get("/health/live", healthHandler.Live)
	r.Get("/health/ready", healthHandler.Ready)

	// Persistent Agent WebSocket Endpoint (Separate from /api/v1 - Protected by Signed HTTP Upgrade)
	r.With(agentAuthMiddleware.Handler).Get("/agent/v1/connect", agentWSHandler.Connect)

	// API Gateway routes (HTTP Request Timeout 30s scoped here)
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Timeout(30 * time.Second))

		// Public Auth Routes
		r.Post("/auth/login", authHandler.Login)
		r.Post("/auth/logout", authHandler.Logout)

		// Public Agent Enrollment Endpoint
		r.Post("/agents/enroll", agentHandler.EnrollAgent)

		// Agent Machine Authenticated Heartbeat Endpoint
		r.Group(func(r chi.Router) {
			r.Use(agentAuthMiddleware.Handler)
			r.Post("/agents/heartbeat", agentHandler.Heartbeat)
		})

		// Protected User Routes (Browser Session)
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.Handler)
			r.Use(custommw.TenantMiddleware)

			r.Get("/auth/session", authHandler.Session)

			// Device Registry Routes (Require device.read permission)
			r.Group(func(r chi.Router) {
				r.Use(custommw.RequirePermission("device.read"))
				r.Get("/devices", deviceHandler.List)
				r.Get("/devices/{id}", deviceHandler.GetByID)
			})

			// Device Stream WebRTC Media Signaling Route (Require device.stream.view permission)
			r.Group(func(r chi.Router) {
				r.Use(custommw.RequirePermission("device.stream.view"))
				r.Get("/devices/{id}/media/ws", browserMediaHandler.ServeHTTP)
			})

			// Control Lease Management Routes (Require device.control.acquire permission)
			r.Group(func(r chi.Router) {
				r.Use(custommw.RequirePermission("device.control.acquire"))
				r.Post("/devices/{id}/control-leases", leaseHandler.AcquireLease)
				r.Post("/devices/{id}/control-leases/{leaseId}/renew", leaseHandler.RenewLease)
				r.Delete("/devices/{id}/control-leases/{leaseId}", leaseHandler.ReleaseLease)
			})

			// Command Dispatch Endpoint (Require device.control.input permission)
			r.Group(func(r chi.Router) {
				r.Use(custommw.RequirePermission("device.control.input"))
				r.Post("/commands", commandHandler.Dispatch)
			})

			// Enrollment Tokens Management Routes (Require agent.enroll permission)
			r.Group(func(r chi.Router) {
				r.Use(custommw.RequirePermission("agent.enroll"))
				r.Post("/enrollment-tokens", agentHandler.CreateToken)
				r.Get("/enrollment-tokens", agentHandler.ListTokens)
				r.Delete("/enrollment-tokens/{id}", agentHandler.RevokeToken)
			})

			r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
				principal, _ := auth.GetPrincipal(r.Context())
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"message": "pong",
					"status":  "active",
					"user_id": principal.UserID,
					"org_id":  principal.OrganizationID,
				})
			})
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
