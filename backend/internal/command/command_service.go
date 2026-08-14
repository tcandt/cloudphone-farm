package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	devservice "github.com/tcandt/cloudphone-farm/backend/internal/device"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

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

func (s *CommandService) DispatchCommand(ctx context.Context, orgID, userID string, req *domain.DispatchCommandRequest) (*domain.DeviceCommand, error) {
	if req.DeviceID == "" {
		return nil, errors.New("missing device_id")
	}
	if req.Type == "" {
		return nil, errors.New("missing command type")
	}
	if req.ControlLeaseID == "" {
		return nil, errors.New("missing control_lease_id")
	}

	// 1. Verify Device Ownership (Tenant Isolation)
	var deviceOrg string
	var deviceStatus string
	err := s.pool.QueryRow(ctx, "SELECT organization_id, status FROM devices WHERE device_id = $1", req.DeviceID).Scan(&deviceOrg, &deviceStatus)
	if err != nil {
		return nil, fmt.Errorf("%w: device %s not found", domain.ErrDeviceNotFound, req.DeviceID)
	}

	if deviceOrg != orgID {
		return nil, domain.ErrUnauthorizedCommand
	}

	if deviceStatus != "online" {
		return nil, fmt.Errorf("%w: device is %s", domain.ErrDeviceOffline, deviceStatus)
	}

	// 2. Validate Command Payload & Type
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
		xNum, isXNum := parseNumber(xVal)
		yNum, isYNum := parseNumber(yVal)
		if !isXNum || xNum < 0 || xNum > 1 {
			return nil, errors.New("normalized coordinate x must be a number between 0.0 and 1.0")
		}
		if !isYNum || yNum < 0 || yNum > 1 {
			return nil, errors.New("normalized coordinate y must be a number between 0.0 and 1.0")
		}

	case "gesture.swipe":
		if req.Payload == nil {
			return nil, errors.New("missing payload for gesture.swipe")
		}
		for _, key := range []string{"startX", "startY", "endX", "endY"} {
			val, ok := req.Payload[key]
			if !ok {
				return nil, fmt.Errorf("missing required %s in gesture.swipe payload", key)
			}
			numVal, isNum := parseNumber(val)
			if !isNum || numVal < 0 || numVal > 1 {
				return nil, fmt.Errorf("normalized coordinate %s must be a number between 0.0 and 1.0", key)
			}
		}
		durationVal, ok := req.Payload["durationMs"]
		if !ok {
			return nil, errors.New("missing required durationMs in gesture.swipe payload")
		}
		durNum, isDurNum := parseNumber(durationVal)
		if !isDurNum || durNum < 50 || durNum > 5000 {
			return nil, errors.New("durationMs must be a number between 50ms and 5000ms")
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
		INSERT INTO commands (
			command_id, device_id, organization_id, actor_id,
			command_type, payload, status, idempotency_key, created_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7, $8, $9)
	`
	idempotencyKey := req.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = fmt.Sprintf("idemp_%s", uuid.New().String()[:12])
	}

	_, err = tx.Exec(ctx, insertCmdSQL, cmdID, req.DeviceID, orgID, userID, req.Type, payloadBytes, idempotencyKey, now, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert command record: %w", err)
	}

	// Insert into command_events audit table
	insertEventSQL := `
		INSERT INTO command_events (
			event_id, command_id, device_id, organization_id,
			status, event_type, payload, created_at
		) VALUES ($1, $2, $3, $4, 'pending', 'created', $5, $6)
	`
	eventID := fmt.Sprintf("evt_%s", uuid.New().String()[:12])
	_, err = tx.Exec(ctx, insertEventSQL, eventID, cmdID, req.DeviceID, orgID, payloadBytes, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert command event record: %w", err)
	}

	// Insert into command_outbox table for asynchronous worker dispatch to agent
	insertOutboxSQL := `
		INSERT INTO command_outbox (
			outbox_id, command_id, device_id, organization_id,
			command_type, payload, status, created_at, scheduled_at
		) VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7, $8)
	`
	outboxID := fmt.Sprintf("out_%s", uuid.New().String()[:12])
	_, err = tx.Exec(ctx, insertOutboxSQL, outboxID, cmdID, req.DeviceID, orgID, req.Type, payloadBytes, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert command outbox record: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit command dispatch transaction: %w", err)
	}

	slog.Info("Dispatched command successfully to transactional outbox",
		"command_id", cmdID,
		"device_id", req.DeviceID,
		"org_id", orgID,
		"type", req.Type,
		"lease_id", lease.ControlLeaseID,
		"fencing_token", lease.FencingToken,
	)

	return &domain.DeviceCommand{
		CommandID:      cmdID,
		DeviceID:       req.DeviceID,
		OrganizationID: orgID,
		ActorID:        userID,
		CommandType:    req.Type,
		Payload:        payloadMap,
		Status:         "pending",
		CreatedAt:      now,
	}, nil
}

func parseNumber(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}
