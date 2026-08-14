package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
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
