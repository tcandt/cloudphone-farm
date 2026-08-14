package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	devservice "github.com/tcandt/cloudphone-farm/backend/internal/device"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

type DispatchRequest struct {
	DeviceID       string                 `json:"deviceId"`
	Type           string                 `json:"type"`
	Payload        map[string]interface{} `json:"payload"`
	ControlLeaseID string                 `json:"controlLeaseId"`
	IdempotencyKey string                 `json:"idempotencyKey"`
}

type CommandService struct {
	pool         *pgxpool.Pool
	leaseService *devservice.LeaseService
}

func NewCommandService(pool *pgxpool.Pool, leaseService *devservice.LeaseService) *CommandService {
	return &CommandService{
		pool:         pool,
		leaseService: leaseService,
	}
}

func (s *CommandService) DispatchCommand(ctx context.Context, orgID, userID string, req DispatchRequest) (*domain.DeviceCommand, error) {
	if s.pool == nil {
		return nil, errors.New("postgres connection pool uninitialized")
	}

	if req.IdempotencyKey == "" {
		req.IdempotencyKey = fmt.Sprintf("idemp_%s", uuid.New().String()[:12])
	}

	// 1. Check Idempotency Store in PostgreSQL before starting new dispatch
	existingQuery := `
		SELECT command_id, device_id, organization_id, actor_id, command_type, status, created_at
		FROM commands
		WHERE organization_id = $1 AND actor_id = $2 AND idempotency_key = $3
	`
	var existing domain.DeviceCommand
	err := s.pool.QueryRow(ctx, existingQuery, orgID, userID, req.IdempotencyKey).Scan(&existing.CommandID, &existing.DeviceID, &existing.OrganizationID, &existing.ActorID, &existing.CommandType, &existing.Status, &existing.CreatedAt)
	if err == nil {
		return &existing, nil // Idempotent return of pre-existing command
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("failed to query idempotency key: %w", err)
	}

	// 2. Strict Input Command Payload Validation
	switch req.Type {
	case "gesture.touch":
		if req.Payload == nil {
			return nil, errors.New("missing payload for gesture.touch")
		}
		xVal, hasX := req.Payload["x"]
		yVal, hasY := req.Payload["y"]
		if !hasX || !hasY {
			return nil, errors.New("missing x or y coordinate in gesture.touch payload")
		}
		if _, isNumX := xVal.(float64); !isNumX {
			if _, isIntX := xVal.(int); !isIntX {
				return nil, errors.New("coordinate x must be a valid number")
			}
		}
		if _, isNumY := yVal.(float64); !isNumY {
			if _, isIntY := yVal.(int); !isIntY {
				return nil, errors.New("coordinate y must be a valid number")
			}
		}

	case "gesture.swipe":
		if req.Payload == nil {
			return nil, errors.New("missing payload for gesture.swipe")
		}
		for _, key := range []string{"startX", "startY", "endX", "endY"} {
			if _, ok := req.Payload[key]; !ok {
				return nil, fmt.Errorf("missing %s in gesture.swipe payload", key)
			}
		}
		if dur, ok := req.Payload["durationMs"]; ok {
			durFloat, isNum := dur.(float64)
			if !isNum || durFloat < 50 || durFloat > 5000 {
				return nil, errors.New("durationMs must be a number between 50ms and 5000ms")
			}
		}

	case "input.text":
		if req.Payload == nil {
			return nil, errors.New("missing payload for input.text")
		}
		txt, ok := req.Payload["text"].(string)
		if !ok || len(txt) == 0 || len(txt) > 1000 {
			return nil, errors.New("invalid or overlong text in input.text payload (1-1000 chars required)")
		}

	case "global.back", "global.home", "global.recents":
		// Valid basic navigation

	default:
		return nil, fmt.Errorf("%w: command type %s not allowed", domain.ErrUnauthorizedCommand, req.Type)
	}

	// 3. Active Lease & Fencing Token Validation
	lease, err := s.leaseService.GetActiveLease(ctx, orgID, req.DeviceID)
	if err != nil {
		return nil, err
	}

	if lease.ControlLeaseID != req.ControlLeaseID || lease.UserID != userID || lease.OrganizationID != orgID {
		return nil, domain.ErrLeaseNotOwned
	}

	if time.Now().After(lease.ExpiresAt) {
		return nil, domain.ErrLeaseNotFound
	}

	// 4. Construct Command Record & Payload
	cmdID := fmt.Sprintf("cmd_%s", uuid.New().String()[:12])
	now := time.Now().UTC()
	expiresAt := now.Add(15 * time.Second)

	payloadMap := req.Payload
	if payloadMap == nil {
		payloadMap = make(map[string]interface{})
	}
	payloadMap["control_lease_id"] = lease.ControlLeaseID
	payloadMap["fencing_token"] = lease.FencingToken

	payloadBytes, err := json.Marshal(payloadMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command payload: %w", err)
	}

	// 5. Single PostgreSQL Transaction (commands + command_events + command_outbox)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin command dispatch transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert into commands table (Includes idempotency_key NOT NULL)
	insertCmdSQL := `
		INSERT INTO commands (command_id, organization_id, device_id, actor_id, command_type, payload, status, idempotency_key, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, 'pending', $7, $8, $9)
	`
	if _, err := tx.Exec(ctx, insertCmdSQL, cmdID, orgID, req.DeviceID, userID, req.Type, string(payloadBytes), req.IdempotencyKey, expiresAt, now); err != nil {
		return nil, fmt.Errorf("failed to insert command into postgres: %w", err)
	}

	// Insert into command_events table
	insertEvtSQL := `
		INSERT INTO command_events (command_id, status, payload)
		VALUES ($1, 'pending', $2::jsonb)
	`
	evtPayload, _ := json.Marshal(map[string]interface{}{
		"actor_id":      userID,
		"fencing_token": lease.FencingToken,
		"timestamp":     now.Format(time.RFC3339),
	})
	if _, err := tx.Exec(ctx, insertEvtSQL, cmdID, string(evtPayload)); err != nil {
		return nil, fmt.Errorf("failed to insert command event: %w", err)
	}

	// Insert into command_outbox table (Corrected BIGSERIAL outbox_id and event_type column)
	insertOutboxSQL := `
		INSERT INTO command_outbox (command_id, organization_id, device_id, event_type, payload, status, created_at)
		VALUES ($1, $2, $3, 'command.dispatch', $4::jsonb, 'pending', $5)
	`
	if _, err := tx.Exec(ctx, insertOutboxSQL, cmdID, orgID, req.DeviceID, string(payloadBytes), now); err != nil {
		return nil, fmt.Errorf("failed to insert outbox record: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit command dispatch transaction: %w", err)
	}

	return &domain.DeviceCommand{
		CommandID:      cmdID,
		DeviceID:       req.DeviceID,
		OrganizationID: orgID,
		ActorID:        userID,
		CommandType:    req.Type,
		Status:         "pending",
		CreatedAt:      now,
	}, nil
}
