package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

type CommandRecord struct {
	CommandID      string
	OrganizationID string
	DeviceID       string
	ActorID        string
	CommandType    string
	PayloadJSON    []byte
	Status         string
	ExpiresAt      time.Time
	CreatedAt      time.Time
	ExecutedAt     *time.Time
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
		SELECT command_id, organization_id, device_id, actor_id, command_type, payload, status, expires_at, created_at, executed_at
		FROM commands
		WHERE command_id = $1
	`
	var c CommandRecord
	err := r.pool.QueryRow(ctx, query, commandID).Scan(&c.CommandID, &c.OrganizationID, &c.DeviceID, &c.ActorID, &c.CommandType, &c.PayloadJSON, &c.Status, &c.ExpiresAt, &c.CreatedAt, &c.ExecutedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrDeviceNotFound
		}
		return nil, fmt.Errorf("failed to fetch command by ID: %w", err)
	}

	return &c, nil
}

// UpdateCommandStatus enforces strict state machine transitions and records command_events
func (r *CommandRepository) UpdateCommandStatus(ctx context.Context, commandID string, newStatus string, errStr string) error {
	if r.pool == nil {
		return errors.New("postgres connection pool uninitialized")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin command update transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Fetch current status FOR UPDATE
	var currentStatus string
	selectSQL := `SELECT status FROM commands WHERE command_id = $1 FOR UPDATE`
	if err := tx.QueryRow(ctx, selectSQL, commandID).Scan(&currentStatus); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrDeviceNotFound
		}
		return fmt.Errorf("failed to query command status for update: %w", err)
	}

	// 2. Validate strict state machine transition
	if err := agentws.ValidateStateTransition(currentStatus, newStatus); err != nil {
		if errors.Is(err, agentws.ErrTerminalStateLocked) || currentStatus == newStatus {
			return nil // Idempotent ignore for terminal or same state
		}
		return err
	}

	// 3. Update status
	var updateSQL string
	if agentws.IsTerminalState(newStatus) {
		updateSQL = `UPDATE commands SET status = $1, error_message = $2, executed_at = CURRENT_TIMESTAMP WHERE command_id = $3`
		_, err = tx.Exec(ctx, updateSQL, newStatus, errStr, commandID)
	} else {
		updateSQL = `UPDATE commands SET status = $1, error_message = $2 WHERE command_id = $3`
		_, err = tx.Exec(ctx, updateSQL, newStatus, errStr, commandID)
	}
	if err != nil {
		return fmt.Errorf("failed to update command status: %w", err)
	}

	// 4. Record command_event
	eventSQL := `
		INSERT INTO command_events (event_id, command_id, status, details)
		VALUES ($1, $2, $3, $4::jsonb)
	`
	eventID := fmt.Sprintf("evt_%d", time.Now().UnixNano())
	evtDetails, _ := json.Marshal(map[string]interface{}{
		"old_status": currentStatus,
		"new_status": newStatus,
		"error":      errStr,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	})
	_, _ = tx.Exec(ctx, eventSQL, eventID, commandID, newStatus, string(evtDetails))

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit command status transaction: %w", err)
	}

	return nil
}
