package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
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
	if req.IdempotencyKey == "" {
		return nil, domain.ErrIdempotencyKeyRequired
	}

	// 1. Verify Device Ownership & Status (Tenant Isolation)
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

	// 2. Validate Command Payload, Coordinate Space, & Type Invariants
	switch req.Type {
	case "gesture.touch", "gesture.swipe":
		if req.Payload == nil {
			return nil, fmt.Errorf("missing payload for %s", req.Type)
		}
		space, _ := req.Payload["coordinateSpace"].(string)
		if space != "normalized_display_v1" {
			return nil, fmt.Errorf("invalid coordinateSpace '%s': required 'normalized_display_v1'", space)
		}
		orient, _ := req.Payload["orientation"].(string)
		if orient != "portrait" && orient != "landscape" {
			return nil, fmt.Errorf("invalid orientation '%s': required 'portrait' or 'landscape'", orient)
		}

		if req.Type == "gesture.touch" {
			xVal, hasX := req.Payload["x"]
			yVal, hasY := req.Payload["y"]
			if !hasX || !hasY {
				return nil, errors.New("missing x or y coordinate in gesture.touch payload")
			}
			xNum, isXNum := parseNumber(xVal)
			yNum, isYNum := parseNumber(yVal)
			if !isXNum || xNum < 0 || xNum > 1 {
				return nil, errors.New("normalized coordinate x must be a finite number between 0.0 and 1.0")
			}
			if !isYNum || yNum < 0 || yNum > 1 {
				return nil, errors.New("normalized coordinate y must be a finite number between 0.0 and 1.0")
			}
		} else {
			for _, key := range []string{"startX", "startY", "endX", "endY"} {
				val, ok := req.Payload[key]
				if !ok {
					return nil, fmt.Errorf("missing required %s in gesture.swipe payload", key)
				}
				numVal, isNum := parseNumber(val)
				if !isNum || numVal < 0 || numVal > 1 {
					return nil, fmt.Errorf("normalized coordinate %s must be a finite number between 0.0 and 1.0", key)
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

	// 3. Fast-path Idempotency Key check against existing commands
	cmd, idempErr := s.handleExistingIdempotentCommand(ctx, orgID, userID, req)
	if idempErr == nil {
		return cmd, nil
	} else if errors.Is(idempErr, domain.ErrIdempotencyConflict) {
		return nil, domain.ErrIdempotencyConflict
	}

	// 4. Active Lease & Fencing Token Validation
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

	// 5. Construct Command Record & Injected Payload
	cmdID := fmt.Sprintf("cmd_%s", uuid.New().String()[:12])
	now := time.Now().UTC()
	expiresAt := now.Add(15 * time.Second)

	payloadMap := make(map[string]interface{})
	if req.Payload != nil {
		for k, v := range req.Payload {
			payloadMap[k] = v
		}
	}
	payloadMap["control_lease_id"] = lease.ControlLeaseID
	payloadMap["fencing_token"] = lease.FencingToken

	payloadBytes, err := json.Marshal(payloadMap)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command payload: %w", err)
	}

	// 6. Execute Single PostgreSQL Transaction (commands + command_events + command_outbox)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin command dispatch transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Insert into commands table
	insertCmdSQL := `
		INSERT INTO commands (
			command_id, device_id, organization_id, actor_id,
			command_type, payload, status, idempotency_key, created_at, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7, $8, $9)
	`
	_, err = tx.Exec(ctx, insertCmdSQL, cmdID, req.DeviceID, orgID, userID, req.Type, payloadBytes, req.IdempotencyKey, now, expiresAt)
	if err != nil {
		// Handle concurrent unique constraint violation on uk_org_actor_idempotency (PostgreSQL code 23505)
		if isUniqueViolation(err) {
			_ = tx.Rollback(ctx)
			return s.handleExistingIdempotentCommand(ctx, orgID, userID, req)
		}
		return nil, fmt.Errorf("failed to insert command record: %w", err)
	}

	// Insert into command_events table (BIGSERIAL event_id auto-generated)
	insertEventSQL := `
		INSERT INTO command_events (
			command_id, status, payload, created_at
		) VALUES ($1, 'pending', $2, $3)
	`
	_, err = tx.Exec(ctx, insertEventSQL, cmdID, payloadBytes, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert command event record: %w", err)
	}

	// Insert into command_outbox table (BIGSERIAL outbox_id auto-generated)
	insertOutboxSQL := `
		INSERT INTO command_outbox (
			command_id, organization_id, device_id, event_type,
			payload, status, created_at
		) VALUES ($1, $2, $3, 'command.dispatch', $4, 'pending', $5)
	`
	_, err = tx.Exec(ctx, insertOutboxSQL, cmdID, orgID, req.DeviceID, payloadBytes, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert command outbox record: %w", err)
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
		Payload:        payloadMap,
		Status:         "pending",
		CreatedAt:      now,
	}, nil
}

func (s *CommandService) handleExistingIdempotentCommand(ctx context.Context, orgID, userID string, req *domain.DispatchCommandRequest) (*domain.DeviceCommand, error) {
	var existingCmdID, existingDeviceID, existingType, existingStatus string
	var existingPayloadBytes []byte
	var existingCreatedAt time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT command_id, device_id, command_type, payload, status, created_at
		FROM commands
		WHERE organization_id = $1 AND actor_id = $2 AND idempotency_key = $3
	`, orgID, userID, req.IdempotencyKey).Scan(&existingCmdID, &existingDeviceID, &existingType, &existingPayloadBytes, &existingStatus, &existingCreatedAt)

	if err != nil {
		return nil, fmt.Errorf("idempotency lookup error: %w", err)
	}

	if existingDeviceID == req.DeviceID && existingType == req.Type && comparePayloadFingerprint(existingPayloadBytes, req.Payload) {
		var unmarshaledPayload map[string]interface{}
		_ = json.Unmarshal(existingPayloadBytes, &unmarshaledPayload)
		return &domain.DeviceCommand{
			CommandID:      existingCmdID,
			DeviceID:       existingDeviceID,
			OrganizationID: orgID,
			ActorID:        userID,
			CommandType:    existingType,
			Payload:        unmarshaledPayload,
			Status:         existingStatus,
			CreatedAt:      existingCreatedAt,
		}, nil
	}

	return nil, domain.ErrIdempotencyConflict
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "23505") || strings.Contains(errStr, "unique constraint") || strings.Contains(errStr, "uk_org_actor_idempotency")
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

func comparePayloadFingerprint(existingPayloadBytes []byte, reqPayload map[string]interface{}) bool {
	var existingMap map[string]interface{}
	if err := json.Unmarshal(existingPayloadBytes, &existingMap); err != nil {
		return false
	}

	cleanExisting := cleanUserPayload(existingMap)
	cleanReq := cleanUserPayload(reqPayload)

	return reflect.DeepEqual(cleanExisting, cleanReq)
}

func cleanUserPayload(input map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	if input == nil {
		return out
	}
	for k, v := range input {
		if k == "control_lease_id" || k == "fencing_token" {
			continue
		}
		if num, ok := parseNumber(v); ok {
			out[k] = num
		} else {
			out[k] = v
		}
	}
	return out
}
