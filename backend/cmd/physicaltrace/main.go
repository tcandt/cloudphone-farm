package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/internal/command"
	devservice "github.com/tcandt/cloudphone-farm/backend/internal/device"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	pgrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
	redisrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
)

type PhysicalTraceResult struct {
	GitSHA              string                   `json:"git_sha"`
	Timestamp           string                   `json:"timestamp"`
	IdempotencyKey      string                   `json:"idempotency_key"`
	ServerCommandID     string                   `json:"server_command_id"`
	LeaseID             string                   `json:"lease_id"`
	FencingToken        int64                    `json:"fencing_token"`
	ServiceResponse     *domain.DeviceCommand    `json:"service_response"`
	DBCommandsRow       map[string]interface{}   `json:"db_commands_row"`
	DBCommandEventsRow  map[string]interface{}   `json:"db_command_events_row"`
	DBCommandOutboxRow  map[string]interface{}   `json:"db_command_outbox_row"`
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://pcp:pcp_password@localhost:5432/phone_farm?sslmode=disable"
	}
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pool.Close()

	// 1. Apply Production SQL Migrations
	migrations := []string{
		"000001_create_core_tables.up.sql",
		"000002_seed_initial_rbac.up.sql",
		"000003_harden_agent_identity_and_enrollment.up.sql",
		"000004_harden_command_outbox.up.sql",
		"000005_harden_command_runtime.up.sql",
		"000006_control_lease_and_command_contract.up.sql",
	}

	migrationsDir := filepath.Join("db", "migrations")
	for _, mFile := range migrations {
		mPath := filepath.Join(migrationsDir, mFile)
		sqlBytes, readErr := os.ReadFile(mPath)
		if readErr != nil {
			log.Fatalf("Failed to read migration file %s: %v", mPath, readErr)
		}
		_, execErr := pool.Exec(ctx, string(sqlBytes))
		if execErr != nil {
			log.Fatalf("Failed to execute migration %s: %v", mFile, execErr)
		}
	}

	// 2. Setup Seed Entities
	orgID := "org_pcp_enterprise_01"
	userID := "usr_op_01"
	deviceID := "dev_s7_001"

	_, _ = pool.Exec(ctx, `
		INSERT INTO organizations (organization_id, name, slug)
		VALUES ($1, 'PCP Enterprise Org', 'pcp-enterprise')
		ON CONFLICT DO NOTHING
	`, orgID)

	_, _ = pool.Exec(ctx, `
		INSERT INTO users (user_id, email, password_hash, display_name)
		VALUES ($1, 'op@pcp.internal', 'hash123', 'Operator User')
		ON CONFLICT DO NOTHING
	`, userID)

	_, _ = pool.Exec(ctx, `
		INSERT INTO devices (device_id, organization_id, status, name, serial_number, model, platform_version)
		VALUES ($1, $2, 'online', 'Samsung Galaxy S7', 'SN12345', 'SM-G930F', '11.0')
		ON CONFLICT DO NOTHING
	`, deviceID, orgID)

	// 3. Acquire Real Control Lease in Postgres & Redis
	rdb := redis.NewClient(&redis.Options{Addr: redisURL})
	fenceRepo := pgrepo.NewFenceRepository(pool)
	leaseRepo := redisrepo.NewLeaseRepository(rdb)
	leaseService := devservice.NewLeaseService(fenceRepo, leaseRepo)

	lease, err := leaseService.AcquireLease(ctx, orgID, deviceID, userID, "Operator User")
	if err != nil {
		log.Fatalf("Failed to acquire control lease: %v", err)
	}

	// 4. Dispatch Command through Command Service
	cmdService := command.NewCommandService(pool, leaseService)
	timestampMs := time.Now().UnixNano() / 1000000
	idempKey := fmt.Sprintf("touch_physical_%d", timestampMs)

	req := &domain.DispatchCommandRequest{
		DeviceID:       deviceID,
		Type:           "gesture.touch",
		ControlLeaseID: lease.ControlLeaseID,
		IdempotencyKey: idempKey,
		Payload: map[string]interface{}{
			"x":               0.5,
			"y":               0.5,
			"coordinateSpace": "normalized_display_v1",
			"orientation":     "portrait",
		},
	}

	cmd, err := cmdService.DispatchCommand(ctx, orgID, userID, req)
	if err != nil {
		log.Fatalf("Failed to dispatch command: %v", err)
	}

	// 5. Query Actual Real Database Rows
	var dbCmdID, dbOrgID, dbDevID, dbActorID, dbType, dbStatus, dbIdempKey string
	var dbCreatedAt time.Time
	err = pool.QueryRow(ctx, `
		SELECT command_id, organization_id, device_id, actor_id, command_type, status, idempotency_key, created_at
		FROM commands WHERE command_id = $1
	`, cmd.CommandID).Scan(&dbCmdID, &dbOrgID, &dbDevID, &dbActorID, &dbType, &dbStatus, &dbIdempKey, &dbCreatedAt)
	if err != nil {
		log.Fatalf("Failed to query commands table: %v", err)
	}

	var eventID int64
	var eventStatus string
	var eventCreatedAt time.Time
	err = pool.QueryRow(ctx, `
		SELECT event_id, status, created_at
		FROM command_events WHERE command_id = $1
	`, cmd.CommandID).Scan(&eventID, &eventStatus, &eventCreatedAt)
	if err != nil {
		log.Fatalf("Failed to query command_events table: %v", err)
	}

	var outboxID int64
	var outboxEventType, outboxStatus string
	var outboxCreatedAt time.Time
	err = pool.QueryRow(ctx, `
		SELECT outbox_id, event_type, status, created_at
		FROM command_outbox WHERE command_id = $1
	`, cmd.CommandID).Scan(&outboxID, &outboxEventType, &outboxStatus, &outboxCreatedAt)
	if err != nil {
		log.Fatalf("Failed to query command_outbox table: %v", err)
	}

	result := &PhysicalTraceResult{
		GitSHA:          cmd.CommandID,
		Timestamp:       time.Now().Format(time.RFC3339),
		IdempotencyKey:  idempKey,
		ServerCommandID: cmd.CommandID,
		LeaseID:         lease.ControlLeaseID,
		FencingToken:    lease.FencingToken,
		ServiceResponse: cmd,
		DBCommandsRow: map[string]interface{}{
			"command_id":      dbCmdID,
			"organization_id": dbOrgID,
			"device_id":       dbDevID,
			"actor_id":        dbActorID,
			"command_type":    dbType,
			"status":          dbStatus,
			"idempotency_key": dbIdempKey,
			"created_at":      dbCreatedAt.Format(time.RFC3339),
		},
		DBCommandEventsRow: map[string]interface{}{
			"event_id":   eventID,
			"command_id": cmd.CommandID,
			"status":     eventStatus,
			"created_at": eventCreatedAt.Format(time.RFC3339),
		},
		DBCommandOutboxRow: map[string]interface{}{
			"outbox_id":       outboxID,
			"command_id":      cmd.CommandID,
			"organization_id": dbOrgID,
			"device_id":       dbDevID,
			"event_type":      outboxEventType,
			"status":          outboxStatus,
			"created_at":      outboxCreatedAt.Format(time.RFC3339),
		},
	}

	jsonBytes, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal JSON result: %v", err)
	}

	fmt.Println(string(jsonBytes))
}
