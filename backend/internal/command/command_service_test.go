package command

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	devservice "github.com/tcandt/cloudphone-farm/backend/internal/device"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	pgrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
	redisrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
)

func TestParseNumber(t *testing.T) {
	num, ok := parseNumber(0.5)
	if !ok || num != 0.5 {
		t.Fatalf("expected 0.5, got %v", num)
	}

	numInt, okInt := parseNumber(100)
	if !okInt || numInt != 100.0 {
		t.Fatalf("expected 100.0, got %v", numInt)
	}

	_, okStr := parseNumber("invalid")
	if okStr {
		t.Fatalf("expected string parsing to fail")
	}
}

func TestComparePayloadFingerprint(t *testing.T) {
	existingBytes := []byte(`{
		"x": 0.5,
		"y": 0.3,
		"coordinateSpace": "normalized_display_v1",
		"orientation": "portrait",
		"control_lease_id": "lease_123",
		"fencing_token": 1
	}`)

	reqPayloadSame := map[string]interface{}{
		"x":               0.5,
		"y":               0.3,
		"coordinateSpace": "normalized_display_v1",
		"orientation":     "portrait",
	}

	if !comparePayloadFingerprint(existingBytes, reqPayloadSame) {
		t.Fatalf("expected payload fingerprint comparison to succeed for identical user payload")
	}

	reqPayloadDiff := map[string]interface{}{
		"x":               0.8,
		"y":               0.3,
		"coordinateSpace": "normalized_display_v1",
		"orientation":     "portrait",
	}

	if comparePayloadFingerprint(existingBytes, reqPayloadDiff) {
		t.Fatalf("expected payload fingerprint comparison to fail for different coordinates")
	}
}

func TestPostgreSQLDatabaseCommandServiceIntegration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("Skipping PostgreSQL integration test: DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	// Ensure schema tables exist
	schemaDDL := `
		CREATE TABLE IF NOT EXISTS devices (
			device_id VARCHAR(64) PRIMARY KEY,
			organization_id VARCHAR(64) NOT NULL,
			status VARCHAR(32) NOT NULL DEFAULT 'offline',
			display_name VARCHAR(128),
			model VARCHAR(128),
			android_version VARCHAR(32),
			serial_number VARCHAR(128),
			group_id VARCHAR(64),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS control_leases (
			control_lease_id VARCHAR(64) PRIMARY KEY,
			device_id VARCHAR(64) NOT NULL,
			organization_id VARCHAR(64) NOT NULL,
			user_id VARCHAR(64) NOT NULL,
			user_display_name VARCHAR(128),
			fencing_token BIGINT NOT NULL DEFAULT 1,
			acquired_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ NOT NULL,
			ttl_seconds INT NOT NULL DEFAULT 30
		);

		CREATE TABLE IF NOT EXISTS commands (
			command_id VARCHAR(64) PRIMARY KEY,
			device_id VARCHAR(64) NOT NULL,
			organization_id VARCHAR(64) NOT NULL,
			actor_id VARCHAR(64) NOT NULL,
			command_type VARCHAR(64) NOT NULL,
			payload JSONB NOT NULL,
			status VARCHAR(32) NOT NULL DEFAULT 'pending',
			idempotency_key VARCHAR(128) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ NOT NULL,
			CONSTRAINT uk_org_actor_idempotency UNIQUE (organization_id, actor_id, idempotency_key)
		);

		CREATE TABLE IF NOT EXISTS command_events (
			event_id BIGSERIAL PRIMARY KEY,
			command_id VARCHAR(64) NOT NULL,
			status VARCHAR(32) NOT NULL,
			payload JSONB NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		CREATE TABLE IF NOT EXISTS command_outbox (
			outbox_id BIGSERIAL PRIMARY KEY,
			command_id VARCHAR(64) NOT NULL,
			organization_id VARCHAR(64) NOT NULL,
			device_id VARCHAR(64) NOT NULL,
			event_type VARCHAR(64) NOT NULL,
			payload JSONB NOT NULL,
			status VARCHAR(32) NOT NULL DEFAULT 'pending',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			processed_at TIMESTAMPTZ
		);
	`
	_, err = pool.Exec(ctx, schemaDDL)
	if err != nil {
		t.Fatalf("failed to create integration schema tables: %v", err)
	}

	// Insert fixture device and control lease
	orgID := "org_test_integration"
	userID := "usr_test_op"
	deviceID := "dev_integration_001"
	leaseID := "lease_integration_101"

	_, _ = pool.Exec(ctx, "DELETE FROM commands WHERE organization_id = $1", orgID)
	_, _ = pool.Exec(ctx, "DELETE FROM control_leases WHERE organization_id = $1", orgID)
	_, _ = pool.Exec(ctx, "DELETE FROM devices WHERE organization_id = $1", orgID)

	_, err = pool.Exec(ctx, `
		INSERT INTO devices (device_id, organization_id, status, display_name, model, android_version, serial_number)
		VALUES ($1, $2, 'online', 'Integration Device', 'Samsung S7', '11.0', 'SN12345')
	`, deviceID, orgID)
	if err != nil {
		t.Fatalf("failed to insert test device: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO control_leases (control_lease_id, device_id, organization_id, user_id, user_display_name, fencing_token, expires_at)
		VALUES ($1, $2, $3, $4, 'Test Operator', 1, NOW() + INTERVAL '1 hour')
	`, leaseID, deviceID, orgID, userID)
	if err != nil {
		t.Fatalf("failed to insert test lease: %v", err)
	}

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: redisURL})

	fenceRepo := pgrepo.NewFenceRepository(pool)
	leaseRepo := redisrepo.NewLeaseRepository(rdb)

	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("Skipping Redis lease test step: Redis unavailable on %s (%v)", redisURL, err)
	}

	leaseService := devservice.NewLeaseService(fenceRepo, leaseRepo)

	// Pre-create active lease in Redis for test
	activeLease := &domain.ControlLease{
		ControlLeaseID:  leaseID,
		DeviceID:        deviceID,
		OrganizationID:  orgID,
		UserID:          userID,
		UserDisplayName: "Test Operator",
		FencingToken:    1,
		AcquiredAt:      time.Now().UTC(),
		ExpiresAt:       time.Now().UTC().Add(1 * time.Hour),
		TTLSeconds:      3600,
	}
	_ = leaseRepo.AcquireLease(ctx, activeLease)

	cmdService := NewCommandService(pool, leaseService)

	// Test 1: Dispatch Touch Command
	idempKey := "touch_integration_key_001"
	req := &domain.DispatchCommandRequest{
		DeviceID:       deviceID,
		Type:           "gesture.touch",
		ControlLeaseID: leaseID,
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
		t.Fatalf("DispatchCommand failed: %v", err)
	}
	if cmd.CommandID == "" || cmd.CommandID == idempKey {
		t.Fatalf("CommandID must be generated by backend UUID and differ from idempotencyKey, got '%s'", cmd.CommandID)
	}

	// Verify DB Row Counts
	var cmdCount, eventCount, outboxCount int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM commands WHERE command_id = $1", cmd.CommandID).Scan(&cmdCount)
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM command_events WHERE command_id = $1", cmd.CommandID).Scan(&eventCount)
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM command_outbox WHERE command_id = $1", cmd.CommandID).Scan(&outboxCount)

	if cmdCount != 1 || eventCount != 1 || outboxCount != 1 {
		t.Fatalf("DB Integration Invariant Violation: expected (1, 1, 1), got (%d, %d, %d)", cmdCount, eventCount, outboxCount)
	}

	// Test 2: Concurrent Retry with Same Idempotency Key
	var wg sync.WaitGroup
	results := make([]*domain.DeviceCommand, 5)
	errs := make([]error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = cmdService.DispatchCommand(ctx, orgID, userID, req)
		}(i)
	}
	wg.Wait()

	for i := 0; i < 5; i++ {
		if errs[i] != nil {
			t.Fatalf("Concurrent retry %d failed: %v", i, errs[i])
		}
		if results[i].CommandID != cmd.CommandID {
			t.Fatalf("Concurrent retry %d returned different command_id '%s' (expected '%s')", i, results[i].CommandID, cmd.CommandID)
		}
	}

	// Assert commands and outbox table counts STILL remain exactly 1 row
	var cmdCountAfter, outboxCountAfter int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM commands WHERE idempotency_key = $1", idempKey).Scan(&cmdCountAfter)
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM command_outbox WHERE command_id = $1", cmd.CommandID).Scan(&outboxCountAfter)

	if cmdCountAfter != 1 || outboxCountAfter != 1 {
		t.Fatalf("Concurrent Idempotency Safety Violation: expected exactly 1 persisted row, got commands=%d outbox=%d", cmdCountAfter, outboxCountAfter)
	}

	// Test 3: Idempotency Conflict with Different Payload
	reqConflict := &domain.DispatchCommandRequest{
		DeviceID:       deviceID,
		Type:           "gesture.touch",
		ControlLeaseID: leaseID,
		IdempotencyKey: idempKey,
		Payload: map[string]interface{}{
			"x":               0.9,
			"y":               0.9,
			"coordinateSpace": "normalized_display_v1",
			"orientation":     "portrait",
		},
	}
	_, errConflict := cmdService.DispatchCommand(ctx, orgID, userID, reqConflict)
	if errConflict == nil || errConflict != domain.ErrIdempotencyConflict {
		t.Fatalf("expected ErrIdempotencyConflict when payload differs, got %v", errConflict)
	}
}
