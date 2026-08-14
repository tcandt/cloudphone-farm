package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

type FenceRepository struct {
	pool *pgxpool.Pool
}

func NewFenceRepository(pool *pgxpool.Pool) *FenceRepository {
	return &FenceRepository{pool: pool}
}

// IncrementFencingToken atomically increments and returns the persistent fencing token for a device
func (r *FenceRepository) IncrementFencingToken(ctx context.Context, orgID, deviceID string) (int64, error) {
	if r.pool == nil {
		return 0, errors.New("postgres connection pool uninitialized")
	}

	query := `
		INSERT INTO device_control_fences (organization_id, device_id, last_fencing_token)
		VALUES ($1, $2, 1)
		ON CONFLICT (organization_id, device_id)
		DO UPDATE SET last_fencing_token = device_control_fences.last_fencing_token + 1, updated_at = CURRENT_TIMESTAMP
		RETURNING last_fencing_token
	`

	var newToken int64
	err := r.pool.QueryRow(ctx, query, orgID, deviceID).Scan(&newToken)
	if err != nil {
		return 0, fmt.Errorf("failed to increment fencing token in PostgreSQL: %w", err)
	}

	return newToken, nil
}

// InsertLeaseAudit records historical audit trail of acquired control leases in PostgreSQL
func (r *FenceRepository) InsertLeaseAudit(ctx context.Context, lease *domain.ControlLease) error {
	if r.pool == nil {
		return nil
	}

	query := `
		INSERT INTO control_leases (control_lease_id, organization_id, device_id, user_id, fencing_token, acquired_at, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (control_lease_id) DO NOTHING
	`

	_, err := r.pool.Exec(ctx, query, lease.ControlLeaseID, lease.OrganizationID, lease.DeviceID, lease.UserID, lease.FencingToken, lease.AcquiredAt, lease.ExpiresAt)
	if err != nil {
		return fmt.Errorf("failed to insert control_leases audit record: %w", err)
	}

	return nil
}

// UpdateLeaseAuditExpiry updates expires_at for lease renewal audit trail in PostgreSQL
func (r *FenceRepository) UpdateLeaseAuditExpiry(ctx context.Context, leaseID string, expiresAt time.Time) error {
	if r.pool == nil {
		return nil
	}

	query := `UPDATE control_leases SET expires_at = $1 WHERE control_lease_id = $2`
	_, err := r.pool.Exec(ctx, query, expiresAt, leaseID)
	if err != nil {
		return fmt.Errorf("failed to update control_leases audit expiry: %w", err)
	}

	return nil
}

// RevokeLeaseAudit records revoked_at timestamp when lease is released
func (r *FenceRepository) RevokeLeaseAudit(ctx context.Context, leaseID string) error {
	if r.pool == nil {
		return nil
	}

	query := `UPDATE control_leases SET revoked_at = CURRENT_TIMESTAMP WHERE control_lease_id = $1`
	_, err := r.pool.Exec(ctx, query, leaseID)
	if err != nil {
		return fmt.Errorf("failed to revoke control_leases audit record: %w", err)
	}

	return nil
}
