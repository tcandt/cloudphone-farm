package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

type CommandRecord struct {
	CommandID          string
	OrganizationID     string
	DeviceID           string
	ActorID            string
	CommandType        string
	PayloadJSON        []byte
	Status             string
	LastStatusSequence int64
	ExpiresAt          time.Time
	CreatedAt          time.Time
	ExecutedAt         *time.Time
}

type CommandRepository struct {
	pool *pgxpool.Pool
}

func NewCommandRepository(pool *pgxpool.Pool) *CommandRepository {
	return &CommandRepository{pool: pool}
}

// GetCommandByID returns a command record by ID
func (r *CommandRepository) GetCommandByID(ctx context.Context, commandID string) (*CommandRecord, error) {
	if r.pool == nil {
		return nil, errors.New("postgres connection pool uninitialized")
	}

	query := `
		SELECT command_id, organization_id, device_id, actor_id, command_type, payload, status, COALESCE(last_status_sequence, 0), COALESCE(expires_at, created_at + INTERVAL '10 minutes'), created_at, executed_at
		FROM commands
		WHERE command_id = $1
	`
	var c CommandRecord
	err := r.pool.QueryRow(ctx, query, commandID).Scan(&c.CommandID, &c.OrganizationID, &c.DeviceID, &c.ActorID, &c.CommandType, &c.PayloadJSON, &c.Status, &c.LastStatusSequence, &c.ExpiresAt, &c.CreatedAt, &c.ExecutedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDeviceNotFound
		}
		return nil, fmt.Errorf("failed to fetch command by ID: %w", err)
	}

	return &c, nil
}

// UpdateCommandStatus enforces strict state machine transitions for internal server updates
func (r *CommandRepository) UpdateCommandStatus(ctx context.Context, commandID string, newStatus string, errStr string, sequence int64) error {
	if r.pool == nil {
		return errors.New("postgres connection pool uninitialized")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin command update transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var currentStatus string
	var lastSeq int64
	selectSQL := `SELECT status, COALESCE(last_status_sequence, 0) FROM commands WHERE command_id = $1 FOR UPDATE`
	if err := tx.QueryRow(ctx, selectSQL, commandID).Scan(&currentStatus, &lastSeq); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrDeviceNotFound
		}
		return fmt.Errorf("failed to query command status for update: %w", err)
	}

	if sequence > 0 && sequence <= lastSeq {
		return nil // Stale sequence ignored
	}

	if err := agentws.ValidateStateTransition(currentStatus, newStatus); err != nil {
		if errors.Is(err, agentws.ErrTerminalStateLocked) || currentStatus == newStatus {
			return nil
		}
		return err
	}

	var updateSQL string
	if agentws.IsTerminalState(newStatus) {
		updateSQL = `UPDATE commands SET status = $1, error_message = $2, last_status_sequence = $3, executed_at = CURRENT_TIMESTAMP WHERE command_id = $4`
		_, err = tx.Exec(ctx, updateSQL, newStatus, errStr, sequence, commandID)
	} else {
		updateSQL = `UPDATE commands SET status = $1, error_message = $2, last_status_sequence = $3 WHERE command_id = $4`
		_, err = tx.Exec(ctx, updateSQL, newStatus, errStr, sequence, commandID)
	}
	if err != nil {
		return fmt.Errorf("failed to update command status: %w", err)
	}

	eventSQL := `
		INSERT INTO command_events (command_id, status, payload)
		VALUES ($1, $2, $3::jsonb)
	`
	evtPayload, _ := json.Marshal(map[string]interface{}{
		"old_status": currentStatus,
		"new_status": newStatus,
		"sequence":   sequence,
		"error":      errStr,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	})
	if _, err := tx.Exec(ctx, eventSQL, commandID, newStatus, string(evtPayload)); err != nil {
		return fmt.Errorf("failed to insert command event log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit command status transaction: %w", err)
	}

	return nil
}

// UpdateCommandStatusFromAgent enforces device ownership (organization_id AND device_id), state transitions, and per-command sequence persistence
func (r *CommandRepository) UpdateCommandStatusFromAgent(ctx context.Context, orgID, deviceID, commandID, newStatus, errStr string, sequence int64) error {
	if r.pool == nil {
		return errors.New("postgres connection pool uninitialized")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin agent command update transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Fetch current status and last_status_sequence WITH DEVICE OWNERSHIP BOUNDARY CHECK FOR UPDATE
	var currentStatus string
	var lastSeq int64
	selectSQL := `
		SELECT status, COALESCE(last_status_sequence, 0)
		FROM commands
		WHERE command_id = $1 AND organization_id = $2 AND device_id = $3
		FOR UPDATE
	`
	if err := tx.QueryRow(ctx, selectSQL, commandID, orgID, deviceID).Scan(&currentStatus, &lastSeq); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: command %s not owned by device %s in org %s", domain.ErrDeviceNotFound, commandID, deviceID, orgID)
		}
		return fmt.Errorf("failed to query device command status for update: %w", err)
	}

	// 2. Per-Command Sequence Protection Check
	if sequence > 0 && sequence <= lastSeq {
		return nil // Stale sequence ignored per-command
	}

	// 3. Validate strict state machine transition
	if err := agentws.ValidateStateTransition(currentStatus, newStatus); err != nil {
		if errors.Is(err, agentws.ErrTerminalStateLocked) || currentStatus == newStatus {
			return nil // Idempotent ignore for terminal or same state
		}
		return err
	}

	// 4. Update status and sequence
	var updateSQL string
	if agentws.IsTerminalState(newStatus) {
		updateSQL = `UPDATE commands SET status = $1, error_message = $2, last_status_sequence = $3, executed_at = CURRENT_TIMESTAMP WHERE command_id = $4`
		_, err = tx.Exec(ctx, updateSQL, newStatus, errStr, sequence, commandID)
	} else {
		updateSQL = `UPDATE commands SET status = $1, error_message = $2, last_status_sequence = $3 WHERE command_id = $4`
		_, err = tx.Exec(ctx, updateSQL, newStatus, errStr, sequence, commandID)
	}
	if err != nil {
		return fmt.Errorf("failed to update command status from agent: %w", err)
	}

	// 5. Record command_event
	eventSQL := `
		INSERT INTO command_events (command_id, status, payload)
		VALUES ($1, $2, $3::jsonb)
	`
	evtPayload, _ := json.Marshal(map[string]interface{}{
		"old_status": currentStatus,
		"new_status": newStatus,
		"sequence":   sequence,
		"error":      errStr,
		"source":     "agent_ws",
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	})
	if _, err := tx.Exec(ctx, eventSQL, commandID, newStatus, string(evtPayload)); err != nil {
		return fmt.Errorf("failed to insert command event log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit agent command status transaction: %w", err)
	}

	return nil
}

// RecordDeliveryAttempt records an immutable command delivery attempt snapshot in postgres
func (r *CommandRepository) RecordDeliveryAttempt(ctx context.Context, orgID, commandID, deviceID string, attemptNo int, agentID, connectionID string, generation int64, status string) error {
	if r.pool == nil {
		return errors.New("postgres connection pool uninitialized")
	}

	query := `
		INSERT INTO command_delivery_attempts (organization_id, command_id, device_id, attempt_no, agent_id, connection_id, generation, status, dispatched_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP)
		ON CONFLICT (command_id, attempt_no) DO NOTHING
	`
	res, err := r.pool.Exec(ctx, query, orgID, commandID, deviceID, attemptNo, agentID, connectionID, generation, status)
	if err != nil {
		return fmt.Errorf("failed to record command delivery attempt: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("command delivery attempt (command_id=%s, attempt_no=%d) already exists and is immutable", commandID, attemptNo)
	}
	return nil
}

// UpdateDeliveryAttemptStatus updates status of a delivery attempt
func (r *CommandRepository) UpdateDeliveryAttemptStatus(ctx context.Context, commandID string, attemptNo int, status string, failureReason string) error {
	if r.pool == nil {
		return errors.New("postgres connection pool uninitialized")
	}

	var res pgconn.CommandTag
	var err error
	if status == "failed" {
		query := `UPDATE command_delivery_attempts SET status = $1, failed_at = CURRENT_TIMESTAMP, failure_reason = $2 WHERE command_id = $3 AND attempt_no = $4 AND status NOT IN ('dispatched', 'executing', 'succeeded')`
		res, err = r.pool.Exec(ctx, query, status, failureReason, commandID, attemptNo)
		if err == nil && res.RowsAffected() == 0 {
			// Delivery attempt was already dispatched/executing/succeeded, preserve ACKed status
			return nil
		}
	} else {
		query := `UPDATE command_delivery_attempts SET status = $1 WHERE command_id = $2 AND attempt_no = $3`
		res, err = r.pool.Exec(ctx, query, status, commandID, attemptNo)
	}
	if err != nil {
		return fmt.Errorf("failed to update delivery attempt status: %w", err)
	}
	if res.RowsAffected() == 0 {
		return fmt.Errorf("no delivery attempt row matching command_id %s attempt_no %d to update status to %s", commandID, attemptNo, status)
	}
	return nil
}

// UpdateCommandStatusFromAgentWithGeneration verifies latest delivery attempt generation matches sender before applying status transition
func (r *CommandRepository) UpdateCommandStatusFromAgentWithGeneration(ctx context.Context, orgID, deviceID, commandID, agentID, connectionID string, generation int64, newStatus, errStr string, sequence int64) error {
	if r.pool == nil {
		return errors.New("postgres connection pool uninitialized")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin agent command update transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Verify latest delivery attempt matches sender generation (FAIL CLOSED)
	// Supports both 'prepared' and 'dispatched' attempts to prevent fast-ACK race
	attemptQuery := `
		SELECT attempt_no, agent_id, connection_id, generation, status
		FROM command_delivery_attempts
		WHERE command_id = $1
		ORDER BY attempt_no DESC
		LIMIT 1
		FOR UPDATE
	`
	var attemptNo int
	var latestAgentID, latestConnID, attemptStatus string
	var latestGen int64
	err = tx.QueryRow(ctx, attemptQuery, commandID).Scan(&attemptNo, &latestAgentID, &latestConnID, &latestGen, &attemptStatus)
	if err != nil {
		return fmt.Errorf("no authoritative delivery attempt found for command %s: %w", commandID, err)
	}

	if attemptStatus != "prepared" && attemptStatus != "dispatched" {
		return fmt.Errorf("invalid delivery attempt status '%s' for command %s (must be prepared or dispatched)", attemptStatus, commandID)
	}

	if latestAgentID != agentID || latestConnID != connectionID || latestGen != generation {
		return fmt.Errorf("stale generation delivery attempt mismatch: command %s was dispatched to gen %d (%s), received status from gen %d (%s)", commandID, latestGen, latestConnID, generation, connectionID)
	}

	// Transactional Fast-ACK Promotion: if status is prepared, promote to dispatched inside the SAME transaction
	if attemptStatus == "prepared" {
		promoteSQL := `
			UPDATE command_delivery_attempts
			SET status = 'dispatched', dispatched_at = CURRENT_TIMESTAMP
			WHERE command_id = $1 AND attempt_no = $2 AND status IN ('prepared', 'dispatched')
		`
		if _, err := tx.Exec(ctx, promoteSQL, commandID, attemptNo); err != nil {
			return fmt.Errorf("failed to promote prepared attempt to dispatched: %w", err)
		}
	}

	// Fetch current status and last_status_sequence WITH DEVICE OWNERSHIP BOUNDARY CHECK FOR UPDATE
	var currentStatus string
	var lastSeq int64
	selectSQL := `
		SELECT status, COALESCE(last_status_sequence, 0)
		FROM commands
		WHERE command_id = $1 AND organization_id = $2 AND device_id = $3
		FOR UPDATE
	`
	if err := tx.QueryRow(ctx, selectSQL, commandID, orgID, deviceID).Scan(&currentStatus, &lastSeq); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: command %s not owned by device %s in org %s", domain.ErrDeviceNotFound, commandID, deviceID, orgID)
		}
		return fmt.Errorf("failed to query device command status for update: %w", err)
	}

	// Per-Command Sequence Protection Check
	if sequence > 0 && sequence <= lastSeq {
		return nil // Stale sequence ignored per-command
	}

	// Validate strict state machine transition
	if err := agentws.ValidateStateTransition(currentStatus, newStatus); err != nil {
		if errors.Is(err, agentws.ErrTerminalStateLocked) || currentStatus == newStatus {
			return nil // Idempotent ignore for terminal or same state
		}
		return err
	}

	// Update status and sequence
	var updateSQL string
	if agentws.IsTerminalState(newStatus) {
		updateSQL = `UPDATE commands SET status = $1, error_message = $2, last_status_sequence = $3, executed_at = CURRENT_TIMESTAMP WHERE command_id = $4`
		_, err = tx.Exec(ctx, updateSQL, newStatus, errStr, sequence, commandID)
	} else {
		updateSQL = `UPDATE commands SET status = $1, error_message = $2, last_status_sequence = $3 WHERE command_id = $4`
		_, err = tx.Exec(ctx, updateSQL, newStatus, errStr, sequence, commandID)
	}
	if err != nil {
		return fmt.Errorf("failed to update command status from agent: %w", err)
	}

	// Record command_event
	eventSQL := `
		INSERT INTO command_events (command_id, status, payload)
		VALUES ($1, $2, $3::jsonb)
	`
	evtPayload, _ := json.Marshal(map[string]interface{}{
		"old_status": currentStatus,
		"new_status": newStatus,
		"sequence":   sequence,
		"error":      errStr,
		"source":     "agent_ws",
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	})
	if _, err := tx.Exec(ctx, eventSQL, commandID, newStatus, string(evtPayload)); err != nil {
		return fmt.Errorf("failed to insert command event log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit agent command status transaction: %w", err)
	}

	return nil
}
