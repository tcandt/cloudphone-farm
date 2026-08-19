package main

import (
	"context"
	"encoding/json"
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
	"github.com/tcandt/cloudphone-farm/backend/internal/agentenrollment"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentkey"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	"github.com/tcandt/cloudphone-farm/backend/internal/cluster"
	"github.com/tcandt/cloudphone-farm/backend/internal/command"
	"github.com/tcandt/cloudphone-farm/backend/internal/config"
	devservice "github.com/tcandt/cloudphone-farm/backend/internal/device"
	pgrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
	redisrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
	"github.com/tcandt/cloudphone-farm/backend/internal/telemetry"
	httptransport "github.com/tcandt/cloudphone-farm/backend/internal/transport/http"
	custommw "github.com/tcandt/cloudphone-farm/backend/internal/transport/http/middleware"
	wstransport "github.com/tcandt/cloudphone-farm/backend/internal/transport/ws"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("Starting Phone Control Platform Backend Server")

	// Load configuration
	cfg := config.LoadConfig()

	// Execute Production Fail-Fast Validation
	if err := config.ValidateProductionConfig(cfg); err != nil {
		slog.Error("FATAL: Production configuration security validation failed", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize PostgreSQL Connection Pool
	var pgPool *pgxpool.Pool
	pool, err := pgxpool.New(ctx, cfg.PostgresURL)
	if err != nil {
		slog.Warn("Failed to create PostgreSQL connection pool", "error", err)
		if cfg.AppEnv == "production" {
			slog.Error("FATAL: PostgreSQL unavailable in production mode")
			os.Exit(1)
		}
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
		if cfg.AppEnv == "production" {
			slog.Error("FATAL: Redis unavailable in production mode")
			os.Exit(1)
		}
	} else {
		rdb = redis.NewClient(opt)
		defer rdb.Close()
		slog.Info("Redis client initialized")
	}

	// Production Fail-Fast Health Checks (Pings)
	if cfg.AppEnv == "production" {
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer pingCancel()

		if pgPool == nil || pgPool.Ping(pingCtx) != nil {
			slog.Error("FATAL: PostgreSQL ping failed in production mode")
			os.Exit(1)
		}
		if rdb == nil || rdb.Ping(pingCtx).Err() != nil {
			slog.Error("FATAL: Redis ping failed in production mode")
			os.Exit(1)
		}
	}

	// Cluster Components (Node Registry, Message Bus, Cluster Router)
	nodeRegistry := cluster.NewNodeRegistry(cfg.NodeID, rdb, 20*time.Second)
	if rdb != nil {
		if err := nodeRegistry.Start(ctx); err != nil {
			slog.Error("Failed to start cluster node registry", "error", err)
			if cfg.AppEnv == "production" {
				os.Exit(1)
			}
		}
		defer nodeRegistry.Shutdown(context.Background())
	}

	messageBus := cluster.NewMessageBus(cfg.NodeID, rdb)
	if rdb != nil {
		defer messageBus.Close()
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
	agentKeyRepo := pgrepo.NewAgentKeyRepository(pgPool)
	enrollV2Repo := pgrepo.NewEnrollmentV2Repository(pgPool)
	challengeStore := agentenrollment.NewChallengeStore(rdb)

	agentConnRepo := redisrepo.NewAgentConnectionRepository(rdb)
	mediaSessionRepo := redisrepo.NewMediaSessionRepository(rdb)
	viewerRepo := redisrepo.NewViewerRepository(rdb)

	// Agent WebSocket Hub & Browser Event Hub & Command Outbox Dispatcher
	wsHub := agentws.NewHub()
	browserHub := agentws.NewBrowserHub()

	clusterRouter := cluster.NewClusterRouter(cfg.NodeID, messageBus, agentConnRepo, mediaSessionRepo, wsHub, browserHub)
	if rdb != nil {
		if err := clusterRouter.Start(ctx); err != nil {
			slog.Error("Failed to start cluster router", "error", err)
			if cfg.AppEnv == "production" {
				os.Exit(1)
			}
		}
	}

	wsHub.SetDistributedMediaRelayer(wstransport.NewClusterMediaRelayer(mediaSessionRepo, agentConnRepo, clusterRouter))

	outboxDispatcher := command.NewOutboxDispatcher(outboxRepo, cmdRepo, wsHub, browserHub)
	outboxDispatcher.SetClusterComponents(agentConnRepo, clusterRouter)
	outboxDispatcher.Start(ctx)
	defer outboxDispatcher.Stop()

	authService := auth.NewAuthService(userRepo, sessionRepo, time.Duration(cfg.SessionTTLSeconds)*time.Second)
	deviceService := devservice.NewDeviceService(deviceRepo)
	agentService := agentservice.NewAgentService(enrollRepo, presenceRepo, rdb)
	agentService.SetClusterBroadcaster(clusterRouter)
	agentService.SetAgentConnectionRepository(agentConnRepo)
	leaseService := devservice.NewLeaseService(fenceRepo, leaseRepo)
	cmdService := command.NewCommandService(pgPool, leaseService)
	agentKeyService := agentkey.NewService(agentKeyRepo)
	enrollV2Service := agentenrollment.NewEnrollmentV2Service(enrollV2Repo, challengeStore)

	// Handlers & Middlewares
	healthHandler := httptransport.NewHealthHandler(pgPool, rdb, outboxDispatcher)
	authHandler := httptransport.NewAuthHandler(authService, cfg)
	deviceHandler := httptransport.NewDeviceHandler(deviceService)
	agentHandler := httptransport.NewAgentHandler(agentService, rdb)
	agentKeyHandler := httptransport.NewAgentKeyHandler(agentKeyService)
	agentEnrollmentV2Handler := httptransport.NewAgentEnrollmentHandlerV2(enrollV2Service)

	agentWSHandler := wstransport.NewAgentWSHandler(wsHub, enrollRepo, cmdRepo, browserHub)
	agentWSHandler.SetClusterComponents(cfg.NodeID, agentConnRepo, clusterRouter)

	browserMediaHandler := wstransport.NewBrowserMediaHandler(wsHub, deviceService, cfg.CorsAllowedOrigins, viewerRepo)
	browserMediaHandler.SetClusterComponents(cfg.NodeID, agentConnRepo, mediaSessionRepo, clusterRouter)

	browserWSHandler := httptransport.NewBrowserWSHandler(browserHub, deviceService, cfg.CorsAllowedOrigins)
	leaseHandler := httptransport.NewLeaseHandler(leaseService)
	rateLimiter := custommw.NewRateLimiter(rdb, cfg.AppEnv)
	commandHandler := httptransport.NewCommandHandler(cmdService, rateLimiter)

	authMiddleware := custommw.NewAuthMiddleware(authService, cfg.SessionCookieName, cfg.AppEnv)
	agentAuthMiddleware := custommw.NewAgentAuthMiddleware(enrollRepo, rdb)

	// Create Chi router
	r := chi.NewRouter()

	// Global Middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(custommw.SecurityHeadersMiddleware)
	r.Use(custommw.CSRFMiddleware(cfg.CorsAllowedOrigins))

	// CORS configuration
	r.Use(cors.Handler(custommw.GetCorsOptions(cfg.CorsAllowedOrigins)))

	// Health Check Handlers (Public)
	r.Get("/health/live", healthHandler.Live)
	r.Get("/health/ready", healthHandler.Ready)

	// Internal Prometheus Telemetry Exporter (Protected in production)
	r.Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if cfg.AppEnv == "production" {
			token := os.Getenv("INTERNAL_METRICS_TOKEN")
			if token == "" {
				http.Error(w, "Metrics endpoint disabled: INTERNAL_METRICS_TOKEN missing in production", http.StatusForbidden)
				return
			}
			if r.Header.Get("X-Internal-Token") != token {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
		}
		telemetry.PrometheusHandler().ServeHTTP(w, r)
	})

	// Persistent Agent WebSocket Endpoint
	r.With(
		rateLimiter.LimitMiddleware(custommw.ScopeWSUpgrade, 20, 5),
		agentAuthMiddleware.Handler,
	).Get("/agent/v1/connect", agentWSHandler.Connect)

	// API Gateway routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(middleware.Timeout(30 * time.Second))
		r.Use(rateLimiter.LimitMiddleware(custommw.ScopeRestAPI, 100, 20))

		// Public Auth Routes
		r.With(rateLimiter.LimitMiddleware(custommw.ScopeLogin, 10, 2)).Post("/auth/login", authHandler.Login)
		r.Post("/auth/logout", authHandler.Logout)

		// Public Agent Enrollment Endpoint (Rate Limited by Enrollment Bucket)
		r.With(rateLimiter.LimitMiddleware(custommw.ScopeEnrollment, 10, 2)).Post("/agents/enroll", agentHandler.EnrollAgent)

		// Agent Machine Authenticated Heartbeat & Decommission Endpoints
		r.Group(func(r chi.Router) {
			r.Use(agentAuthMiddleware.Handler)
			r.Post("/agents/heartbeat", agentHandler.Heartbeat)
			r.Post("/agents/{agentId}/decommission", agentHandler.Decommission)
		})

		// Protected User Routes (Browser Session)
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.Handler)
			r.Use(custommw.TenantMiddleware)

			r.Get("/auth/session", authHandler.Session)

			// Device Registry Routes
			r.Group(func(r chi.Router) {
				r.Use(custommw.RequirePermission("device.read"))
				r.Get("/devices", deviceHandler.List)
				r.Get("/devices/{id}", deviceHandler.GetByID)
			})

			// Device Stream & Event WebSocket Routes
			r.Group(func(r chi.Router) {
				r.Use(custommw.RequirePermission("device.stream.view"))
				r.With(rateLimiter.LimitMiddleware(custommw.ScopeWSUpgrade, 20, 5)).Get("/devices/{id}/media/ws", browserMediaHandler.ServeHTTP)
				r.With(rateLimiter.LimitMiddleware(custommw.ScopeWSUpgrade, 20, 5)).Get("/devices/{id}/events/ws", browserWSHandler.ServeHTTP)
			})

			// Control Lease Management Routes
			r.Group(func(r chi.Router) {
				r.Use(custommw.RequirePermission("device.control.acquire"))
				r.Post("/devices/{id}/control-leases", leaseHandler.AcquireLease)
				r.Post("/devices/{id}/control-leases/{leaseId}/renew", leaseHandler.RenewLease)
				r.Delete("/devices/{id}/control-leases/{leaseId}", leaseHandler.ReleaseLease)
			})

			// Command Dispatch Endpoint
			r.Group(func(r chi.Router) {
				r.Use(custommw.RequirePermission("device.control.input"))
				r.Post("/commands", commandHandler.Dispatch)
			})

			// Agent & Enrollment Tokens Management Routes
			r.Group(func(r chi.Router) {
				r.Use(custommw.RequireAnyPermission("agent.enroll", "device.read"))
				r.Get("/agents", agentHandler.ListAgents)
			})

			r.Group(func(r chi.Router) {
				r.Use(custommw.RequirePermission("agent.enroll"))
				r.Post("/enrollment-tokens", agentHandler.CreateToken)
				r.Get("/enrollment-tokens", agentHandler.ListTokens)
				r.Get("/enrollment-tokens/{id}/readiness", agentHandler.GetTokenReadiness)
				r.Delete("/enrollment-tokens/{id}", agentHandler.RevokeToken)
				r.Delete("/agents/{agentId}", agentHandler.RevokeAgentCredential)
				r.Post("/admin/agents/{agentId}/decommission", agentHandler.DecommissionByUser)
			})

			r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
				principal, _ := auth.GetPrincipal(r.Context())
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"status":          "pong",
					"user_id":         principal.UserID,
					"organization_id": principal.OrganizationID,
				})
			})
		})
	})

	r.Route("/api/v2", func(r chi.Router) {
		r.Use(middleware.Timeout(30 * time.Second))
		r.Use(rateLimiter.LimitMiddleware(custommw.ScopeRestAPI, 100, 20))

		// Public Agent Enrollment Challenge & Finalize Endpoints (V2)
		r.With(rateLimiter.LimitMiddleware(custommw.ScopeEnrollment, 10, 2)).Route("/agents/enroll", agentEnrollmentV2Handler.RegisterRoutes)

		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.Handler)
			r.Use(custommw.TenantMiddleware)

			r.Group(func(r chi.Router) {
				r.Use(custommw.RequirePermission("agent.enroll"))
				r.Route("/agent-keys", agentKeyHandler.RegisterRoutes)
			})
		})
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		slog.Info("HTTP Gateway & WebSocket Server listening", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server listen failed", "error", err)
		}
	}()

	// Wait for SIGINT/SIGTERM
	<-ctx.Done()
	slog.Info("Shutdown signal received. Initiating graceful shutdown...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("Server forced shutdown error", "error", err)
	}

	slog.Info("Server stopped cleanly")
}
