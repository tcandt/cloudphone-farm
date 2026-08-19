package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	"github.com/tcandt/cloudphone-farm/backend/internal/repository"
)

type agentKeyRepository struct {
	db *pgxpool.Pool
}

func NewAgentKeyRepository(db *pgxpool.Pool) repository.AgentKeyRepository {
	return &agentKeyRepository{db: db}
}

func (r *agentKeyRepository) Create(ctx context.Context, key *domain.AgentKey) error {
	query := `
		INSERT INTO agent_enrollment_keys 
		(key_id, organization_id, created_by, name, token_hash, token_prefix, max_bindings, expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.Exec(ctx, query,
		key.KeyID,
		key.OrganizationID,
		key.CreatedBy,
		key.Name,
		key.TokenHash,
		key.TokenPrefix,
		key.MaxBindings,
		key.ExpiresAt,
		key.CreatedAt,
		key.UpdatedAt,
	)
	return err
}

func (r *agentKeyRepository) GetByID(ctx context.Context, orgID, keyID string) (*domain.AgentKey, error) {
	query := `
		SELECT key_id, organization_id, created_by, name, token_hash, token_prefix, max_bindings, expires_at, revoked_at, created_at, updated_at, last_used_at
		FROM agent_enrollment_keys
		WHERE organization_id = $1 AND key_id = $2
	`
	row := r.db.QueryRow(ctx, query, orgID, keyID)
	
	var key domain.AgentKey
	var maxBindings sql.NullInt64
	var expiresAt, revokedAt, lastUsedAt sql.NullTime

	err := row.Scan(
		&key.KeyID,
		&key.OrganizationID,
		&key.CreatedBy,
		&key.Name,
		&key.TokenHash,
		&key.TokenPrefix,
		&maxBindings,
		&expiresAt,
		&revokedAt,
		&key.CreatedAt,
		&key.UpdatedAt,
		&lastUsedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Or a specific NotFound error based on existing conventions
		}
		return nil, err
	}

	if maxBindings.Valid {
		val := int(maxBindings.Int64)
		key.MaxBindings = &val
	}
	if expiresAt.Valid {
		key.ExpiresAt = &expiresAt.Time
	}
	if revokedAt.Valid {
		key.RevokedAt = &revokedAt.Time
	}
	if lastUsedAt.Valid {
		key.LastUsedAt = &lastUsedAt.Time
	}

	return &key, nil
}

func (r *agentKeyRepository) List(ctx context.Context, orgID string) ([]*domain.AgentKey, error) {
	query := `
		SELECT key_id, organization_id, created_by, name, token_hash, token_prefix, max_bindings, expires_at, revoked_at, created_at, updated_at, last_used_at
		FROM agent_enrollment_keys
		WHERE organization_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []*domain.AgentKey
	for rows.Next() {
		var key domain.AgentKey
		var maxBindings sql.NullInt64
		var expiresAt, revokedAt, lastUsedAt sql.NullTime

		err := rows.Scan(
			&key.KeyID,
			&key.OrganizationID,
			&key.CreatedBy,
			&key.Name,
			&key.TokenHash,
			&key.TokenPrefix,
			&maxBindings,
			&expiresAt,
			&revokedAt,
			&key.CreatedAt,
			&key.UpdatedAt,
			&lastUsedAt,
		)
		if err != nil {
			return nil, err
		}

		if maxBindings.Valid {
			val := int(maxBindings.Int64)
			key.MaxBindings = &val
		}
		if expiresAt.Valid {
			key.ExpiresAt = &expiresAt.Time
		}
		if revokedAt.Valid {
			key.RevokedAt = &revokedAt.Time
		}
		if lastUsedAt.Valid {
			key.LastUsedAt = &lastUsedAt.Time
		}
		keys = append(keys, &key)
	}
	return keys, nil
}

func (r *agentKeyRepository) Update(ctx context.Context, orgID, keyID string, name *string, maxBindings *int, updateMaxBindings bool, expiresAt *time.Time, updateExpiresAt bool) (*domain.AgentKey, error) {
	if name == nil && !updateMaxBindings && !updateExpiresAt {
		return r.GetByID(ctx, orgID, keyID)
	}

	setFields := []string{"updated_at = NOW()"}
	args := []interface{}{orgID, keyID}
	argID := 3

	if name != nil {
		setFields = append(setFields, fmt.Sprintf("name = $%d", argID))
		args = append(args, *name)
		argID++
	}
	if updateMaxBindings {
		setFields = append(setFields, fmt.Sprintf("max_bindings = $%d", argID))
		args = append(args, maxBindings)
		argID++
	}
	if updateExpiresAt {
		setFields = append(setFields, fmt.Sprintf("expires_at = $%d", argID))
		args = append(args, expiresAt)
		argID++
	}

	query := fmt.Sprintf(`
		UPDATE agent_enrollment_keys
		SET %s
		WHERE organization_id = $1 AND key_id = $2 AND revoked_at IS NULL
		RETURNING key_id, organization_id, created_by, name, token_hash, token_prefix, max_bindings, expires_at, revoked_at, created_at, updated_at, last_used_at
	`, strings.Join(setFields, ", "))

	row := r.db.QueryRow(ctx, query, args...)
	
	var outKey domain.AgentKey
	var maxBindingsOut sql.NullInt64
	var expiresAtOut, revokedAt, lastUsedAt sql.NullTime

	err := row.Scan(
		&outKey.KeyID,
		&outKey.OrganizationID,
		&outKey.CreatedBy,
		&outKey.Name,
		&outKey.TokenHash,
		&outKey.TokenPrefix,
		&maxBindingsOut,
		&expiresAtOut,
		&revokedAt,
		&outKey.CreatedAt,
		&outKey.UpdatedAt,
		&lastUsedAt,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil 
		}
		return nil, err
	}

	if maxBindingsOut.Valid {
		val := int(maxBindingsOut.Int64)
		outKey.MaxBindings = &val
	}
	if expiresAtOut.Valid {
		outKey.ExpiresAt = &expiresAtOut.Time
	}
	if revokedAt.Valid {
		outKey.RevokedAt = &revokedAt.Time
	}
	if lastUsedAt.Valid {
		outKey.LastUsedAt = &lastUsedAt.Time
	}

	return &outKey, nil
}

func (r *agentKeyRepository) Revoke(ctx context.Context, orgID, keyID string) error {
	query := `
		UPDATE agent_enrollment_keys
		SET revoked_at = NOW(), updated_at = NOW()
		WHERE organization_id = $1 AND key_id = $2 AND revoked_at IS NULL
	`
	tag, err := r.db.Exec(ctx, query, orgID, keyID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("not found or already revoked")
	}
	return nil
}
