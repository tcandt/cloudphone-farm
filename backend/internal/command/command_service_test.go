package command

import (
	"context"
	"os"
	"path/filepath"
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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	// Apply real project SQL migrations
	migrations := []string{
		"000001_create_core_tables.up.sql",
		"000002_seed_initial_rbac.up.sql",
		"000003_harden_agent_identity_and_enrollment.up.sql",
		"000004_harden_command_outbox.up.sql",
		"000005_harden_command_runtime.up.sql",
		"000006_control_lease_and_command_contract.up.sql",
		"000007_phase14_command_delivery_attempts.up.sql",
	}

	migrationsDir := filepath.Join("..", "..", "db", "migrations")
	for _, mFile := range migrations {
		mPath := filepath.Join(migrationsDir, mFile)
		sqlBytes, readErr := os.ReadFile(mPath)
		if readErr != nil {
			t.Fatalf("failed to read migration file %s: %v", mPath, readErr)
		}
		_, execErr := pool.Exec(ctx, string(sqlBytes))
		if execErr != nil {
			t.Fatalf("failed to execute migration %s: %v", mFile, execErr)
		}
	}

	orgID := "org_test_integration"
	userID := "usr_test_op"
	deviceID := "dev_integration_001"
	leaseID := "lease_integration_101"

	// Cleanup test tables before assertions
	_, _ = pool.Exec(ctx, "DELETE FROM commands WHERE organization_id = $1", orgID)
	_, _ = pool.Exec(ctx, "DELETE FROM control_leases WHERE organization_id = $1", orgID)
	_, _ = pool.Exec(ctx, "DELETE FROM devices WHERE organization_id = $1", orgID)

	// Insert tenant organization first (foreign key dependency)
	_, _ = pool.Exec(ctx, `
		INSERT INTO organizations (organization_id, name, slug)
		VALUES ($1, 'Integration Test Org', 'org-test-integration')
		ON CONFLICT DO NOTHING
	`, orgID)

	// Insert user (foreign key dependency for control_leases)
	_, _ = pool.Exec(ctx, `
		INSERT INTO users (user_id, email, password_hash, display_name)
		VALUES ($1, 'operator@example.com', 'hash123', 'Test Operator')
		ON CONFLICT DO NOTHING
	`, userID)

	_, err = pool.Exec(ctx, `
		INSERT INTO devices (device_id, organization_id, status, name, serial_number, model, platform_version)
		VALUES ($1, $2, 'online', 'Integration Device', 'SN12345', 'Samsung S7', '11.0')
	`, deviceID, orgID)
	if err != nil {
		t.Fatalf("failed to insert test device: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO control_leases (control_lease_id, device_id, organization_id, user_id, fencing_token, expires_at)
		VALUES ($1, $2, $3, $4, 1, NOW() + INTERVAL '1 hour')
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

	// Test 1: Clean Concurrent Idempotency Race (All goroutines start from clean key at barrier)
	raceIdempKey := "race_touch_clean_key_99"
	reqRace := &domain.DispatchCommandRequest{
		DeviceID:       deviceID,
		Type:           "gesture.touch",
		ControlLeaseID: leaseID,
		IdempotencyKey: raceIdempKey,
		Payload: map[string]interface{}{
			"x":               0.5,
			"y":               0.5,
			"coordinateSpace": "normalized_display_v1",
			"orientation":     "portrait",
		},
	}

	const concurrentGoroutines = 5
	var startBarrier sync.WaitGroup
	startBarrier.Add(1)

	var wg sync.WaitGroup
	results := make([]*domain.DeviceCommand, concurrentGoroutines)
	errs := make([]error, concurrentGoroutines)

	for i := 0; i < concurrentGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			startBarrier.Wait() // Wait for barrier trigger so all 5 goroutines run simultaneously
			results[idx], errs[idx] = cmdService.DispatchCommand(ctx, orgID, userID, reqRace)
		}(i)
	}

	startBarrier.Done() // Trigger all goroutines at once
	wg.Wait()

	var firstCmdID string
	for i := 0; i < concurrentGoroutines; i++ {
		if errs[i] != nil {
			t.Fatalf("Concurrent race goroutine %d failed: %v", i, errs[i])
		}
		if i == 0 {
			firstCmdID = results[i].CommandID
		} else {
			if results[i].CommandID != firstCmdID {
				t.Fatalf("Concurrent race goroutine %d returned different command_id '%s' (expected '%s')", i, results[i].CommandID, firstCmdID)
			}
		}
	}

	// Assert EXACT DB Counts
	var cmdCount, eventCount, outboxCount int
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM commands WHERE idempotency_key = $1", raceIdempKey).Scan(&cmdCount)
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM command_events WHERE command_id = $1", firstCmdID).Scan(&eventCount)
	_ = pool.QueryRow(ctx, "SELECT COUNT(*) FROM command_outbox WHERE command_id = $1", firstCmdID).Scan(&outboxCount)

	if cmdCount != 1 || eventCount != 1 || outboxCount != 1 {
		t.Fatalf("Concurrent Race DB Invariant Violation: expected (1, 1, 1), got commands=%d events=%d outbox=%d", cmdCount, eventCount, outboxCount)
	}

	// Test 2: Concurrent Same Key + Different Payload -> ErrIdempotencyConflict
	reqConflict := &domain.DispatchCommandRequest{
		DeviceID:       deviceID,
		Type:           "gesture.touch",
		ControlLeaseID: leaseID,
		IdempotencyKey: raceIdempKey,
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

	// Test 3: Existing Command Status Authority (Set status to 'succeeded' in DB, retry key, assert status == 'succeeded')
	_, errUpdate := pool.Exec(ctx, "UPDATE commands SET status = 'succeeded' WHERE command_id = $1", firstCmdID)
	if errUpdate != nil {
		t.Fatalf("failed to update command status in test: %v", errUpdate)
	}

	retryCmd, errRetry := cmdService.DispatchCommand(ctx, orgID, userID, reqRace)
	if errRetry != nil {
		t.Fatalf("retry on succeeded command failed: %v", errRetry)
	}
	if retryCmd.Status != "succeeded" {
		t.Fatalf("expected returned DeviceCommand.Status to equal 'succeeded', got '%s'", retryCmd.Status)
	}
}

func TestPostgresFastACKAndDeliveryAuthorityFailureMatrix(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("Skipping PostgreSQL integration test: DATABASE_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	cmdRepo := pgrepo.NewCommandRepository(pool)
	orgID := "org_test_fastack"
	deviceID := "dev_test_fastack"
	agentID := "agent_test_fastack"
	connID := "conn_gen_1"
	gen := int64(1001)

	// Clean tables
	_, _ = pool.Exec(ctx, "DELETE FROM command_delivery_attempts WHERE organization_id = $1", orgID)
	_, _ = pool.Exec(ctx, "DELETE FROM command_events WHERE command_id IN (SELECT command_id FROM commands WHERE organization_id = $1)", orgID)
	_, _ = pool.Exec(ctx, "DELETE FROM commands WHERE organization_id = $1", orgID)

	// 1. Test Fast-ACK Promotion: PREPARED -> DISPATCHED when valid ACK arrives before outbox update
	cmd1ID := "cmd_fastack_001"
	_, err = pool.Exec(ctx, "INSERT INTO commands (command_id, organization_id, device_id, actor_id, command_type, payload_json, status, expires_at) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)",
		cmd1ID, orgID, deviceID, "actor_1", "global.home", "{}", "pending", time.Now().Add(10*time.Minute))
	if err != nil {
		t.Fatalf("failed to insert test command 1: %v", err)
	}

	err = cmdRepo.RecordDeliveryAttempt(ctx, orgID, cmd1ID, deviceID, 1, agentID, connID, gen, "prepared")
	if err != nil {
		t.Fatalf("failed to record prepared attempt: %v", err)
	}

	// Fast ACK arrives: status = executing, sequence = 2
	err = cmdRepo.UpdateCommandStatusFromAgentWithGeneration(ctx, orgID, deviceID, cmd1ID, agentID, connID, gen, "executing", "", 2)
	if err != nil {
		t.Fatalf("Fast ACK failed to promote prepared attempt to dispatched: %v", err)
	}

	// Verify attempt status in DB is now 'dispatched'
	var attemptStatus string
	err = pool.QueryRow(ctx, "SELECT status FROM command_delivery_attempts WHERE command_id = $1 AND attempt_no = 1", cmd1ID).Scan(&attemptStatus)
	if err != nil || attemptStatus != "dispatched" {
		t.Fatalf("expected attempt status 'dispatched', got '%s' (err=%v)", attemptStatus, err)
	}

	// 2. Test No-Attempt Status Rejection (fail closed)
	cmd2ID := "cmd_fastack_002"
	_, _ = pool.Exec(ctx, "INSERT INTO commands (command_id, organization_id, device_id, actor_id, command_type, payload_json, status, expires_at) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)",
		cmd2ID, orgID, deviceID, "actor_1", "global.home", "{}", "pending", time.Now().Add(10*time.Minute))

	err = cmdRepo.UpdateCommandStatusFromAgentWithGeneration(ctx, orgID, deviceID, cmd2ID, agentID, connID, gen, "executing", "", 1)
	if err == nil {
		t.Fatalf("expected error when no delivery attempt exists, got nil")
	}

	// 3. Test Failed-Attempt Status Rejection (fail closed)
	cmd3ID := "cmd_fastack_003"
	_, _ = pool.Exec(ctx, "INSERT INTO commands (command_id, organization_id, device_id, actor_id, command_type, payload_json, status, expires_at) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)",
		cmd3ID, orgID, deviceID, "actor_1", "global.home", "{}", "pending", time.Now().Add(10*time.Minute))
	_ = cmdRepo.RecordDeliveryAttempt(ctx, orgID, cmd3ID, deviceID, 1, agentID, connID, gen, "prepared")
	_ = cmdRepo.UpdateDeliveryAttemptStatus(ctx, cmd3ID, 1, "failed", "socket_error")

	err = cmdRepo.UpdateCommandStatusFromAgentWithGeneration(ctx, orgID, deviceID, cmd3ID, agentID, connID, gen, "executing", "", 1)
	if err == nil {
		t.Fatalf("expected error when delivery attempt status is 'failed', got nil")
	}

	// 4. Test Generation Mismatch Rejection
	cmd4ID := "cmd_fastack_004"
	_, _ = pool.Exec(ctx, "INSERT INTO commands (command_id, organization_id, device_id, actor_id, command_type, payload_json, status, expires_at) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, $8)",
		cmd4ID, orgID, deviceID, "actor_1", "global.home", "{}", "pending", time.Now().Add(10*time.Minute))
	_ = cmdRepo.RecordDeliveryAttempt(ctx, orgID, cmd4ID, deviceID, 1, agentID, connID, gen, "dispatched")

	err = cmdRepo.UpdateCommandStatusFromAgentWithGeneration(ctx, orgID, deviceID, cmd4ID, agentID, "conn_gen_2", 1002, "executing", "", 1)
	if err == nil {
		t.Fatalf("expected error on generation mismatch, got nil")
	}

	// 5. Test UpdateDeliveryAttemptStatus RowsAffected == 1 assertion
	err = cmdRepo.UpdateDeliveryAttemptStatus(ctx, "non_existent_cmd", 99, "dispatched", "")
	if err == nil {
		t.Fatalf("expected error updating status of non-existent attempt row, got nil")
	}
}

