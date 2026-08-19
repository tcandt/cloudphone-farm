package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	redispkg "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
)

func setupTestViewerRepo(t *testing.T) (*redispkg.ViewerRepository, *miniredis.Miniredis, *redis.Client) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	repo := redispkg.NewViewerRepository(rdb)
	return repo, mr, rdb
}

func TestViewerRepository_AcquireQuota(t *testing.T) {
	ctx := context.Background()
	repo, _, rdb := setupTestViewerRepo(t)
	defer rdb.Close()

	orgID := "org1"
	deviceID := "dev1"
	ttl := 15 * time.Minute

	t.Run("1. Distinct sessions enforce max viewer quota", func(t *testing.T) {
		session1 := "sess_1"
		session2 := "sess_2"

		err := repo.AcquireViewerLease(ctx, orgID, deviceID, session1, ttl)
		if err != nil {
			t.Fatalf("Expected session1 to acquire lease, got %v", err)
		}

		err = repo.AcquireViewerLease(ctx, orgID, deviceID, session2, ttl)
		if err != redispkg.ErrViewerQuotaExceeded {
			t.Fatalf("Expected session2 to fail with ErrViewerQuotaExceeded, got %v", err)
		}
	})
}

func TestViewerRepository_IdempotentRenewal(t *testing.T) {
	ctx := context.Background()
	repo, _, rdb := setupTestViewerRepo(t)
	defer rdb.Close()

	orgID := "org2"
	deviceID := "dev2"
	sessionID := "sess_renew"
	ttl := 15 * time.Minute

	t.Run("2. Same session can idempotently renew without increasing quota", func(t *testing.T) {
		err := repo.AcquireViewerLease(ctx, orgID, deviceID, sessionID, ttl)
		if err != nil {
			t.Fatalf("Expected initial acquire to succeed, got %v", err)
		}

		// Re-acquire should succeed
		err = repo.AcquireViewerLease(ctx, orgID, deviceID, sessionID, ttl)
		if err != nil {
			t.Fatalf("Expected re-acquire to succeed, got %v", err)
		}

		count, err := repo.GetActiveViewerCount(ctx, orgID, deviceID)
		if err != nil {
			t.Fatalf("GetActiveViewerCount failed: %v", err)
		}
		if count != 1 {
			t.Fatalf("Expected active viewer count to be 1, got %d", count)
		}
	})
}

func TestViewerRepository_ReleaseAndReacquire(t *testing.T) {
	ctx := context.Background()
	repo, _, rdb := setupTestViewerRepo(t)
	defer rdb.Close()

	orgID := "org3"
	deviceID := "dev3"
	session1 := "sess_1"
	session2 := "sess_2"
	ttl := 15 * time.Minute

	t.Run("3. Releasing lease allows new session to acquire", func(t *testing.T) {
		err := repo.AcquireViewerLease(ctx, orgID, deviceID, session1, ttl)
		if err != nil {
			t.Fatalf("Expected session1 to acquire lease, got %v", err)
		}

		err = repo.ReleaseViewerLease(ctx, orgID, deviceID, session1)
		if err != nil {
			t.Fatalf("Expected session1 to release lease, got %v", err)
		}

		err = repo.AcquireViewerLease(ctx, orgID, deviceID, session2, ttl)
		if err != nil {
			t.Fatalf("Expected session2 to acquire lease after session1 release, got %v", err)
		}
	})
}

func TestViewerRepository_Isolation(t *testing.T) {
	ctx := context.Background()
	repo, _, rdb := setupTestViewerRepo(t)
	defer rdb.Close()

	session1 := "sess_1"
	session2 := "sess_2"
	ttl := 15 * time.Minute

	t.Run("4. Quota keys are isolated by organization and device", func(t *testing.T) {
		// Acquire for device 1
		err := repo.AcquireViewerLease(ctx, "orgA", "dev1", session1, ttl)
		if err != nil {
			t.Fatalf("Expected acquire for orgA/dev1 to succeed, got %v", err)
		}

		// Different device in same org should succeed with new session
		err = repo.AcquireViewerLease(ctx, "orgA", "dev2", session2, ttl)
		if err != nil {
			t.Fatalf("Expected acquire for orgA/dev2 to succeed, got %v", err)
		}

		// Different org, same device ID should succeed with new session
		err = repo.AcquireViewerLease(ctx, "orgB", "dev1", session2, ttl)
		if err != nil {
			t.Fatalf("Expected acquire for orgB/dev1 to succeed, got %v", err)
		}
	})
}
