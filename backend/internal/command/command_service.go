package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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

	// 1. Strict Input Command Scoping (Only allow touch/swipe/text/back/home/recents)
	switch req.Type {
	case "gesture.touch":
		if req.Payload == nil {
			return nil, errors.New("missing payload for gesture.touch")
		}
		if _, ok := req.Payload["x"]; !ok {
			return nil, errors.New("missing coordinate x in touch payload")
		}
		if _, ok := req.Payload["y"]; !ok {
			return nil, errors.New("missing coordinate y in touch payload")
		}
	case "gesture.swipe":
		if req.Payload == nil {
			return nil, errors.New("missing payload for gesture.swipe")
		}
		if _, ok := req.Payload["startX"]; !ok {
			return nil, errors.New("missing startX in swipe payload")
		}
		if _, ok := req.Payload["endX"]; !ok {
			return nil, errors.New("missing endX in swipe payload")
		}
	case "input.text":
		if req.Payload == nil {
			return nil, errors.New("missing payload for input.text")
		}
		txt, ok := req.Payload["text"].(string)
		if !ok || len(txt) > 1000 {
			return nil, errors.New("invalid or overlong text in input.text payload")
		}
	case "global.back", "global.home", "global.recents":
		// Valid basic navigation
	default:
		// Reject sensitive administrative commands in input endpoint
		return nil, fmt.Errorf("%w: command type %s not allowed", domain.ErrUnauthorizedCommand, req.Type)
	}

	// 2. Active Lease & Fencing Token Validation
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

	// 3. Construct Command & Outbox Records
	cmdID := fmt.Sprintf("cmd_%s", uuid.New().String()[:12])
	outboxID := fmt.Sprintf("box_%s", uuid.New().String()[:12])
	now := time.Now().UTC()
	expiresAt := now.Add(15 * time.Second) // Input command TTL: 15s

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

	// 4. Single PostgreSQL Transaction (commands + command_events + command_outbox)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin command dispatch transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert into commands table
	insertCmdSQL := `
		INSERT INTO commands (command_id, organization_id, device_id, actor_id, command_type, payload, status, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, 'pending', $7, $8)
	`
	if _, err := tx.Exec(ctx, insertCmdSQL, cmdID, orgID, req.DeviceID, userID, req.Type, string(payloadBytes), expiresAt, now); err != nil {
		return nil, fmt.Errorf("failed to insert command: %w", err)
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

	// Insert into command_outbox table
	insertOutboxSQL := `
		INSERT INTO command_outbox (outbox_id, command_id, device_id, organization_id, payload, status, created_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, 'pending', $8)
	`
	if _, err := tx.Exec(ctx, insertOutboxSQL, outboxID, cmdID, req.DeviceID, orgID, string(payloadBytes), now); err != nil {
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
