package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

var (
	ErrTokenInvalidOrConsumed = errors.New("enrollment token is invalid, expired, revoked, or already consumed")
)

type EnrollmentTokenRecord struct {
	TokenID        string
	OrganizationID string
	TokenHash      string
	CreatedBy      string
	BoundGroupID   *string
	ExpiresAt      time.Time
	CreatedAt      time.Time
	ConsumedAt     *time.Time
	RevokedAt      *time.Time
}

type EnrollmentTokenMetadata struct {
	TokenID        string    `json:"token_id"`
	OrganizationID string    `json:"organization_id"`
	CreatedBy      string    `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	Status         string    `json:"status"` // active, consumed, revoked, expired
	BoundGroupID   *string   `json:"bound_group_id,omitempty"`
}

type AgentEnrollmentPayload struct {
	TokenCode            string
	TokenHash            string
	PublicKeyBytes       []byte
	PublicKeyFingerprint string
	ApkVersion           string
	ProtocolVersion      string
	DeviceSerialNumber   string
	DeviceModel          string
	DeviceAndroidVersion string
	DeviceDisplayName    string
	CapabilitiesJSON     []byte
	CorrelationID        string
}

type EnrollmentResult struct {
	AgentID        string `json:"agent_id"`
	DeviceID       string `json:"device_id"`
	OrganizationID string `json:"organization_id"`
}

type EnrollmentRepository struct {
	pool *pgxpool.Pool
}

func NewEnrollmentRepository(pool *pgxpool.Pool) *EnrollmentRepository {
	return &EnrollmentRepository{pool: pool}
}

// CreateToken inserts a new enrollment token record by token_hash
func (r *EnrollmentRepository) CreateToken(ctx context.Context, rec EnrollmentTokenRecord) error {
	if r.pool == nil {
		return errors.New("postgres connection pool uninitialized")
	}

	query := `
		INSERT INTO enrollment_tokens (token_id, organization_id, token_hash, created_by, bound_group_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.pool.Exec(ctx, query, rec.TokenID, rec.OrganizationID, rec.TokenHash, rec.CreatedBy, rec.BoundGroupID, rec.ExpiresAt, rec.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create enrollment token: %w", err)
	}

	return nil
}

// ListTokens returns metadata for all organization enrollment tokens (never returns raw token_code!)
func (r *EnrollmentRepository) ListTokens(ctx context.Context, orgID string) ([]EnrollmentTokenMetadata, error) {
	if r.pool == nil {
		return nil, errors.New("postgres connection pool uninitialized")
	}

	query := `
		SELECT token_id, organization_id, created_by, created_at, expires_at, bound_group_id, consumed_at, revoked_at
		FROM enrollment_tokens
		WHERE organization_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.pool.Query(ctx, query, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to query enrollment tokens: %w", err)
	}
	defer rows.Close()

	var items []EnrollmentTokenMetadata
	now := time.Now()

	for rows.Next() {
		var m EnrollmentTokenMetadata
		var consumed, revoked *time.Time

		if err := rows.Scan(&m.TokenID, &m.OrganizationID, &m.CreatedBy, &m.CreatedAt, &m.ExpiresAt, &m.BoundGroupID, &consumed, &revoked); err != nil {
			return nil, fmt.Errorf("failed to scan token metadata: %w", err)
		}

		if revoked != nil {
			m.Status = "revoked"
		} else if consumed != nil {
			m.Status = "consumed"
		} else if now.After(m.ExpiresAt) {
			m.Status = "expired"
		} else {
			m.Status = "active"
		}

		items = append(items, m)
	}

	if items == nil {
		items = []EnrollmentTokenMetadata{}
	}

	return items, nil
}

// RevokeToken marks an active enrollment token as revoked
func (r *EnrollmentRepository) RevokeToken(ctx context.Context, orgID, tokenID string) error {
	if r.pool == nil {
		return errors.New("postgres connection pool uninitialized")
	}

	query := `
		UPDATE enrollment_tokens
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE organization_id = $1 AND token_id = $2 AND revoked_at IS NULL AND consumed_at IS NULL
	`
	res, err := r.pool.Exec(ctx, query, orgID, tokenID)
	if err != nil {
		return fmt.Errorf("failed to revoke enrollment token: %w", err)
	}
	if res.RowsAffected() == 0 {
		return domain.ErrDeviceNotFound
	}

	return nil
}

// ConsumeTokenAndRegisterDeviceAgent executes atomic FOR UPDATE redemption and device/agent registration
func (r *EnrollmentRepository) ConsumeTokenAndRegisterDeviceAgent(ctx context.Context, payload AgentEnrollmentPayload) (*EnrollmentResult, error) {
	if r.pool == nil {
		return nil, errors.New("postgres connection pool uninitialized")
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin enrollment transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. SELECT ... FOR UPDATE (Atomic One-time Consumption)
	selectQuery := `
		SELECT token_id, organization_id, bound_group_id, created_by
		FROM enrollment_tokens
		WHERE token_hash = $1
		  AND consumed_at IS NULL
		  AND revoked_at IS NULL
		  AND expires_at > CURRENT_TIMESTAMP
		FOR UPDATE
	`

	var tokenID, orgID, createdBy string
	var boundGroupID *string

	err = tx.QueryRow(ctx, selectQuery, payload.TokenHash).Scan(&tokenID, &orgID, &boundGroupID, &createdBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTokenInvalidOrConsumed
		}
		return nil, fmt.Errorf("token selection failed: %w", err)
	}

	// 2. Register / Upsert Device
	deviceID := fmt.Sprintf("dev_%s", payload.DeviceSerialNumber[:min(12, len(payload.DeviceSerialNumber))])
	deviceSQL := `
		INSERT INTO devices (device_id, organization_id, group_id, name, serial_number, model, platform_version, status, capabilities)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'online', $8)
		ON CONFLICT (organization_id, device_id) DO UPDATE SET
			name = EXCLUDED.name,
			model = EXCLUDED.model,
			platform_version = EXCLUDED.platform_version,
			status = 'online',
			capabilities = EXCLUDED.capabilities,
			updated_at = CURRENT_TIMESTAMP
	`
	_, err = tx.Exec(ctx, deviceSQL, deviceID, orgID, boundGroupID, payload.DeviceDisplayName, payload.DeviceSerialNumber, payload.DeviceModel, payload.DeviceAndroidVersion, payload.CapabilitiesJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert device record: %w", err)
	}

	// 3. Register / Upsert Device Agent (ON CONFLICT target fix: agent_id)
	agentID := fmt.Sprintf("agt_%s", payload.PublicKeyFingerprint[:min(12, len(payload.PublicKeyFingerprint))])
	agentSQL := `
		INSERT INTO device_agents (agent_id, organization_id, device_id, public_key, public_key_fingerprint, apk_version, protocol_version, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'active')
		ON CONFLICT (agent_id) DO UPDATE SET
			public_key = EXCLUDED.public_key,
			public_key_fingerprint = EXCLUDED.public_key_fingerprint,
			apk_version = EXCLUDED.apk_version,
			protocol_version = EXCLUDED.protocol_version,
			status = 'active',
			last_authenticated_at = CURRENT_TIMESTAMP
	`
	_, err = tx.Exec(ctx, agentSQL, agentID, orgID, deviceID, payload.PublicKeyBytes, payload.PublicKeyFingerprint, payload.ApkVersion, payload.ProtocolVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert device_agents record: %w", err)
	}

	// 4. Mark token as consumed
	consumeSQL := `UPDATE enrollment_tokens SET consumed_at = CURRENT_TIMESTAMP WHERE token_id = $1`
	_, err = tx.Exec(ctx, consumeSQL, tokenID)
	if err != nil {
		return nil, fmt.Errorf("failed to mark enrollment token as consumed: %w", err)
	}

	// 5. Audit Log (Schema compliant insert with correlation_id and JSONB details)
	auditSQL := `
		INSERT INTO audit_logs (organization_id, actor_id, correlation_id, action, resource_type, resource_id, details)
		VALUES ($1, $2, $3, 'agent.enroll', 'device_agent', $4, $5::jsonb)
	`
	detailsMap := map[string]interface{}{
		"message":   fmt.Sprintf("Agent enrolled for device %s", deviceID),
		"device_id": deviceID,
		"model":     payload.DeviceModel,
		"serial":    payload.DeviceSerialNumber,
	}
	detailsJSON, _ := json.Marshal(detailsMap)

	corrID := payload.CorrelationID
	if corrID == "" {
		corrID = fmt.Sprintf("cor_%d", time.Now().UnixNano())
	}

	if _, err := tx.Exec(ctx, auditSQL, orgID, createdBy, corrID, agentID, string(detailsJSON)); err != nil {
		return nil, fmt.Errorf("failed to insert enrollment audit log: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit enrollment transaction: %w", err)
	}

	return &EnrollmentResult{
		AgentID:        agentID,
		DeviceID:       deviceID,
		OrganizationID: orgID,
	}, nil
}

// GetAgentByID fetches device agent details including PublicKey by agent_id
func (r *EnrollmentRepository) GetAgentByID(ctx context.Context, agentID string) (*domain.DeviceAgent, error) {
	if r.pool == nil {
		return nil, errors.New("postgres connection pool uninitialized")
	}

	query := `
		SELECT agent_id, organization_id, device_id, public_key, COALESCE(public_key_fingerprint, ''), apk_version, status, revoked_at
		FROM device_agents
		WHERE agent_id = $1
	`
	var a domain.DeviceAgent
	var revoked *time.Time

	err := r.pool.QueryRow(ctx, query, agentID).Scan(&a.AgentID, &a.OrganizationID, &a.DeviceID, &a.PublicKey, &a.PublicKeyFingerprint, &a.ApkVersion, &a.Status, &revoked)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("agent not found")
		}
		return nil, fmt.Errorf("failed to fetch agent by ID: %w", err)
	}

	if revoked != nil || a.Status == "revoked" {
		return nil, errors.New("agent credential has been revoked")
	}

	return &a, nil
}

// GetAgentByFingerprint fetches device agent details by public key fingerprint
func (r *EnrollmentRepository) GetAgentByFingerprint(ctx context.Context, fingerprint string) (*domain.DeviceAgent, error) {
	if r.pool == nil {
		return nil, errors.New("postgres connection pool uninitialized")
	}

	query := `
		SELECT agent_id, organization_id, device_id, public_key, COALESCE(public_key_fingerprint, ''), apk_version, status, revoked_at
		FROM device_agents
		WHERE public_key_fingerprint = $1
	`
	var a domain.DeviceAgent
	var revoked *time.Time

	err := r.pool.QueryRow(ctx, query, fingerprint).Scan(&a.AgentID, &a.OrganizationID, &a.DeviceID, &a.PublicKey, &a.PublicKeyFingerprint, &a.ApkVersion, &a.Status, &revoked)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.New("agent not found")
		}
		return nil, fmt.Errorf("failed to fetch agent by fingerprint: %w", err)
	}

	if revoked != nil || a.Status == "revoked" {
		return nil, errors.New("agent credential has been revoked")
	}

	return &a, nil
}

// RecordDeviceHeartbeat inserts a telemetry snapshot into PostgreSQL device_heartbeats table
func (r *EnrollmentRepository) RecordDeviceHeartbeat(ctx context.Context, orgID, deviceID string, cpu, ram, temp float64, battery int, network string) error {
	if r.pool == nil {
		return nil
	}

	query := `
		INSERT INTO device_heartbeats (organization_id, device_id, cpu_usage, memory_usage, battery_level, temperature_c, network_type, received_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP)
	`
	_, err := r.pool.Exec(ctx, query, orgID, deviceID, cpu, ram, battery, temp, network)
	if err != nil {
		return fmt.Errorf("failed to record device heartbeat telemetry in PostgreSQL: %w", err)
	}

	return nil
}
