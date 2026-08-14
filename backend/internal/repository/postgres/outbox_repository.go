package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxMessage struct {
	OutboxID       string
	CommandID      string
	DeviceID       string
	OrganizationID string
	PayloadJSON    []byte
	Status         string
	AttemptCount   int
	CreatedAt      time.Time
}

type OutboxRepository struct {
	pool *pgxpool.Pool
}

func NewOutboxRepository(pool *pgxpool.Pool) *OutboxRepository {
	return &OutboxRepository{pool: pool}
}

// ClaimPendingOutboxMessages claims a batch of pending outbox messages using FOR UPDATE SKIP LOCKED
func (r *OutboxRepository) ClaimPendingOutboxMessages(ctx context.Context, workerID string, limit int) ([]OutboxMessage, error) {
	if r.pool == nil {
		return nil, errors.New("postgres connection pool uninitialized")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin outbox claim transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	selectQuery := `
		SELECT outbox_id, command_id, device_id, organization_id, payload, status, attempt_count, created_at
		FROM command_outbox
		WHERE status = 'pending'
		  AND (next_attempt_at IS NULL OR next_attempt_at <= CURRENT_TIMESTAMP)
		ORDER BY created_at ASC
		FOR UPDATE SKIP LOCKED
		LIMIT $1
	`

	rows, err := tx.Query(ctx, selectQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending outbox messages: %w", err)
	}
	defer rows.Close()

	var messages []OutboxMessage
	for rows.Next() {
		var msg OutboxMessage
		if err := rows.Scan(&msg.OutboxID, &msg.CommandID, &msg.DeviceID, &msg.OrganizationID, &msg.PayloadJSON, &msg.Status, &msg.AttemptCount, &msg.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan outbox message: %w", err)
		}
		messages = append(messages, msg)
	}
	rows.Close()

	if len(messages) == 0 {
		return []OutboxMessage{}, nil
	}

	// Mark batch as claimed and release DB transaction
	for _, msg := range messages {
		updateSQL := `
			UPDATE command_outbox
			SET status = 'claimed', locked_by = $1, locked_at = CURRENT_TIMESTAMP
			WHERE outbox_id = $2
		`
		if _, err := tx.Exec(ctx, updateSQL, workerID, msg.OutboxID); err != nil {
			return nil, fmt.Errorf("failed to mark outbox message claimed: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit outbox claim transaction: %w", err)
	}

	return messages, nil
}

// UpdateOutboxDispatched marks outbox message as dispatched (server dispatched to WS hub, command execution pending ACK)
func (r *OutboxRepository) UpdateOutboxDispatched(ctx context.Context, outboxID string) error {
	if r.pool == nil {
		return errors.New("postgres connection pool uninitialized")
	}

	query := `
		UPDATE command_outbox
		SET status = 'dispatched', dispatched_at = CURRENT_TIMESTAMP, locked_by = NULL, locked_at = NULL
		WHERE outbox_id = $1
	`
	_, err := r.pool.Exec(ctx, query, outboxID)
	if err != nil {
		return fmt.Errorf("failed to update outbox dispatched status: %w", err)
	}

	return nil
}

// UpdateOutboxRetry schedules next retry attempt with exponential backoff and records last_error
func (r *OutboxRepository) UpdateOutboxRetry(ctx context.Context, outboxID string, errStr string, nextAttemptAt time.Time) error {
	if r.pool == nil {
		return errors.New("postgres connection pool uninitialized")
	}

	query := `
		UPDATE command_outbox
		SET status = 'pending',
		    attempt_count = attempt_count + 1,
		    last_attempt_at = CURRENT_TIMESTAMP,
		    last_error = $1,
		    next_attempt_at = $2,
		    locked_by = NULL,
		    locked_at = NULL
		WHERE outbox_id = $3
	`
	_, err := r.pool.Exec(ctx, query, errStr, nextAttemptAt, outboxID)
	if err != nil {
		return fmt.Errorf("failed to update outbox retry: %w", err)
	}

	return nil
}

// UpdateOutboxFailed marks outbox message as permanently failed after max retries or TTL expiration
func (r *OutboxRepository) UpdateOutboxFailed(ctx context.Context, outboxID string, errStr string) error {
	if r.pool == nil {
		return errors.New("postgres connection pool uninitialized")
	}

	query := `
		UPDATE command_outbox
		SET status = 'failed',
		    last_attempt_at = CURRENT_TIMESTAMP,
		    last_error = $1,
		    locked_by = NULL,
		    locked_at = NULL
		WHERE outbox_id = $2
	`
	_, err := r.pool.Exec(ctx, query, errStr, outboxID)
	if err != nil {
		return fmt.Errorf("failed to update outbox failed: %w", err)
	}

	return nil
}
