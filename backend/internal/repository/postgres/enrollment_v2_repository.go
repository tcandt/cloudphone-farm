package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentenrollment"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

type enrollmentV2Repository struct {
	pool *pgxpool.Pool
}

func NewEnrollmentV2Repository(pool *pgxpool.Pool) agentenrollment.EnrollmentV2Repository {
	return &enrollmentV2Repository{
		pool: pool,
	}
}

func (r *enrollmentV2Repository) BeginTx(ctx context.Context) (agentenrollment.EnrollmentTx, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &enrollmentTx{tx: tx}, nil
}

func (r *enrollmentV2Repository) GetTokenHashByPrefix(ctx context.Context, tokenPrefix string) (string, error) {
	// Token prefix is NOT an authority in V2 challenge, we use token_hash from sha256(raw).
	// This method might not be used since we hash the raw token in the service.
	return "", errors.New("not implemented")
}

func (r *enrollmentV2Repository) ResolveKey(ctx context.Context, tokenHash string) (*agentenrollment.EnrollmentKey, error) {
	const query = `
		SELECT key_id, organization_id, max_bindings, expires_at, revoked_at
		FROM agent_enrollment_keys
		WHERE token_hash = $1
	`
	row := r.pool.QueryRow(ctx, query, tokenHash)
	
	var k agentenrollment.EnrollmentKey
	var maxBindings sql.NullInt64
	var expiresAt, revokedAt sql.NullTime
	
	if err := row.Scan(&k.KeyID, &k.OrganizationID, &maxBindings, &expiresAt, &revokedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Return nil, nil for not found (service checks it)
		}
		return nil, err
	}
	
	if maxBindings.Valid {
		val := int(maxBindings.Int64)
		k.MaxBindings = &val
	}
	if expiresAt.Valid {
		k.ExpiresAt = &expiresAt.Time
	}
	if revokedAt.Valid {
		k.RevokedAt = &revokedAt.Time
	}
	
	return &k, nil
}

type enrollmentTx struct {
	tx pgx.Tx
}

func (t *enrollmentTx) Commit(ctx context.Context) error {
	return t.tx.Commit(ctx)
}

func (t *enrollmentTx) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

func (t *enrollmentTx) RevalidateKey(ctx context.Context, organizationID, keyID, tokenHash string) (*agentenrollment.EnrollmentKey, error) {
	const query = `
		SELECT key_id, organization_id, max_bindings, expires_at, revoked_at
		FROM agent_enrollment_keys
		WHERE organization_id = $1 AND key_id = $2 AND token_hash = $3
		FOR UPDATE
	`
	row := t.tx.QueryRow(ctx, query, organizationID, keyID, tokenHash)
	
	var k agentenrollment.EnrollmentKey
	var maxBindings sql.NullInt64
	var expiresAt, revokedAt sql.NullTime
	
	if err := row.Scan(&k.KeyID, &k.OrganizationID, &maxBindings, &expiresAt, &revokedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	
	if maxBindings.Valid {
		val := int(maxBindings.Int64)
		k.MaxBindings = &val
	}
	if expiresAt.Valid {
		k.ExpiresAt = &expiresAt.Time
	}
	if revokedAt.Valid {
		k.RevokedAt = &revokedAt.Time
	}
	
	return &k, nil
}

func (t *enrollmentTx) CheckIdempotency(ctx context.Context, organizationID, clientInstanceID string) (*agentenrollment.IdempotencyResult, error) {
	const query = `
		SELECT device_id, agent_id, public_key_fingerprint, status, revoked_at
		FROM device_agents
		WHERE organization_id = $1 AND client_instance_id = $2
	`
	row := t.tx.QueryRow(ctx, query, organizationID, clientInstanceID)
	
	res := &agentenrollment.IdempotencyResult{Exists: true}
	var revokedAt sql.NullTime
	if err := row.Scan(&res.DeviceID, &res.AgentID, &res.Fingerprint, &res.Status, &revokedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &agentenrollment.IdempotencyResult{Exists: false}, nil
		}
		return nil, err
	}
	if revokedAt.Valid {
		res.RevokedAt = &revokedAt.Time
	}
	return res, nil
}

func (t *enrollmentTx) CountActiveBindings(ctx context.Context, organizationID, keyID string) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM agent_key_bindings
		WHERE organization_id = $1 AND key_id = $2 AND released_at IS NULL
	`
	var count int
	if err := t.tx.QueryRow(ctx, query, organizationID, keyID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (t *enrollmentTx) CreateBinding(ctx context.Context, device *domain.Device, agent *domain.DeviceAgent, binding *domain.AgentKeyBinding) error {
	// 1. Create Device
	const qDevice = `
		INSERT INTO devices (
			device_id, organization_id, name, serial_number, model, platform_version, status, capabilities
		) VALUES ($1, $2, $3, $4, $5, $6, $7, '{}'::jsonb)
	`
	if _, err := t.tx.Exec(ctx, qDevice,
		device.DeviceID, device.OrganizationID, device.Name, device.SerialNumber, device.Model, device.PlatformVersion, device.Status,
	); err != nil {
		return err
	}

	// 2. Create Device Agent
	const qAgent = `
		INSERT INTO device_agents (
			agent_id, organization_id, device_id, client_instance_id, public_key, public_key_fingerprint, apk_version, protocol_version, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if _, err := t.tx.Exec(ctx, qAgent,
		agent.AgentID, agent.OrganizationID, agent.DeviceID, agent.ClientInstanceID, agent.PublicKey, agent.PublicKeyFingerprint, agent.ApkVersion, agent.ProtocolVersion, agent.Status,
	); err != nil {
		return err
	}

	// 3. Create Binding
	const qBinding = `
		INSERT INTO agent_key_bindings (
			binding_id, organization_id, key_id, device_id, agent_id, public_key_fingerprint, bound_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	if _, err := t.tx.Exec(ctx, qBinding,
		binding.BindingID, binding.OrganizationID, binding.KeyID, binding.DeviceID, binding.AgentID, binding.PublicKeyFingerprint, binding.BoundAt,
	); err != nil {
		return err
	}

	return nil
}

func (t *enrollmentTx) UpdateKeyLastUsedAt(ctx context.Context, organizationID, keyID string) error {
	const query = `
		UPDATE agent_enrollment_keys
		SET last_used_at = CURRENT_TIMESTAMP
		WHERE organization_id = $1 AND key_id = $2
	`
	_, err := t.tx.Exec(ctx, query, organizationID, keyID)
	return err
}
