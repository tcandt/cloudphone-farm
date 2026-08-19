package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

func TestAgentKeyRepository_Integration(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = os.Getenv("POSTGRES_URL")
	}
	if dbURL == "" {
		t.Skip("Skipping PostgreSQL integration test: DATABASE_URL and POSTGRES_URL not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err)
	defer pool.Close()

	repo := NewAgentKeyRepository(pool)

	orgID := "org_test_1"
	orgID2 := "org_test_2"

	_, err = pool.Exec(ctx, "INSERT INTO organizations (organization_id, name, slug, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW()) ON CONFLICT DO NOTHING", orgID, "Test Org 1", "test-org-1")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO organizations (organization_id, name, slug, created_at, updated_at) VALUES ($1, $2, $3, NOW(), NOW()) ON CONFLICT DO NOTHING", orgID2, "Test Org 2", "test-org-2")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "INSERT INTO users (user_id, email, password_hash, display_name, created_at, updated_at) VALUES ($1, $2, 'hash', 'Test User', NOW(), NOW()) ON CONFLICT DO NOTHING", "user_1", "test1@example.com")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO users (user_id, email, password_hash, display_name, created_at, updated_at) VALUES ($1, $2, 'hash', 'Test User 2', NOW(), NOW()) ON CONFLICT DO NOTHING", "user_2", "test2@example.com")
	require.NoError(t, err)

	t.Run("Create and GetByID", func(t *testing.T) {
		key := &domain.AgentKey{
			KeyID:          "key_1",
			OrganizationID: orgID,
			CreatedBy:      "user_1",
			Name:           "Test Key",
			TokenHash:      "hash",
			TokenPrefix:    "prefix",
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}

		err := repo.Create(ctx, key)
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, orgID, "key_1")
		require.NoError(t, err)
		assert.Equal(t, "key_1", fetched.KeyID)
		assert.Equal(t, "Test Key", fetched.Name)
		assert.Equal(t, 0, fetched.ActiveBindings) // Should be 0 initially

		// Tenant isolation check
		fetchedWrongOrg, err := repo.GetByID(ctx, orgID2, "key_1")
		require.NoError(t, err)
		assert.Nil(t, fetchedWrongOrg)
	})

	t.Run("List and Tenant Isolation", func(t *testing.T) {
		keys, err := repo.List(ctx, orgID)
		require.NoError(t, err)
		assert.NotEmpty(t, keys)

		keys2, err := repo.List(ctx, orgID2)
		require.NoError(t, err)
		assert.Empty(t, keys2) // Should be empty for org 2
	})

	t.Run("Update and Revoke", func(t *testing.T) {
		key := &domain.AgentKey{
			KeyID:          "key_2",
			OrganizationID: orgID,
			CreatedBy:      "user_1",
			Name:           "Test Key 2",
			TokenHash:      "hash2",
			TokenPrefix:    "prefix2",
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}

		err := repo.Create(ctx, key)
		require.NoError(t, err)

		newName := "Updated Name"
		updated, err := repo.Update(ctx, orgID, "key_2", &newName, nil, false, nil, false)
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", updated.Name)

		err = repo.Revoke(ctx, orgID, "key_2")
		require.NoError(t, err)

		fetched, err := repo.GetByID(ctx, orgID, "key_2")
		require.NoError(t, err)
		assert.NotNil(t, fetched.RevokedAt)
	})
}
