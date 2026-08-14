package device

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	pgrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
	redisrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
)

type LeaseService struct {
	fenceRepo *pgrepo.FenceRepository
	leaseRepo *redisrepo.LeaseRepository
}

func NewLeaseService(fenceRepo *pgrepo.FenceRepository, leaseRepo *redisrepo.LeaseRepository) *LeaseService {
	return &LeaseService{
		fenceRepo: fenceRepo,
		leaseRepo: leaseRepo,
	}
}

func (s *LeaseService) AcquireLease(ctx context.Context, orgID, deviceID, userID, userDisplayName string) (*domain.ControlLease, error) {
	// 1. Monotonic atomic increment of fencing token in PostgreSQL
	fencingToken, err := s.fenceRepo.IncrementFencingToken(ctx, orgID, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to increment fencing token: %w", err)
	}

	leaseID := fmt.Sprintf("lease_%s", uuid.New().String()[:12])
	now := time.Now().UTC()
	expiresAt := now.Add(30 * time.Second)

	lease := &domain.ControlLease{
		ControlLeaseID:  leaseID,
		DeviceID:        deviceID,
		OrganizationID:  orgID,
		UserID:          userID,
		UserDisplayName: userDisplayName,
		FencingToken:    fencingToken,
		AcquiredAt:      now,
		ExpiresAt:       expiresAt,
		TTLSeconds:      30,
	}

	// 2. Set exclusive lease in Redis with CAS NX check (Realtime authority)
	if err := s.leaseRepo.AcquireLease(ctx, lease); err != nil {
		return nil, err
	}

	// 3. Persist historical audit record in PostgreSQL (Compensates Redis if audit fails)
	if err := s.fenceRepo.InsertLeaseAudit(ctx, lease); err != nil {
		slog.Error("Failed to record control lease audit in PostgreSQL, compensating Redis lease", "err", err, "lease_id", leaseID)
		_ = s.leaseRepo.ReleaseLease(ctx, orgID, deviceID, leaseID, userID, fencingToken)
		return nil, fmt.Errorf("failed to record control lease audit: %w", err)
	}

	return lease, nil
}

func (s *LeaseService) RenewLease(ctx context.Context, orgID, deviceID, leaseID, userID string) (*domain.ControlLease, error) {
	existing, err := s.leaseRepo.GetLease(ctx, orgID, deviceID)
	if err != nil {
		return nil, err
	}

	if existing.ControlLeaseID != leaseID || existing.UserID != userID {
		return nil, domain.ErrLeaseNotOwned
	}

	renewedLease, err := s.leaseRepo.RenewLease(ctx, orgID, deviceID, leaseID, userID, existing.FencingToken)
	if err != nil {
		return nil, err
	}

	// Update historical audit record expiry in PostgreSQL
	if err := s.fenceRepo.UpdateLeaseAuditExpiry(ctx, leaseID, renewedLease.ExpiresAt); err != nil {
		slog.Error("Failed to update control lease audit expiry", "err", err, "lease_id", leaseID)
		return nil, fmt.Errorf("failed to update lease audit expiry: %w", err)
	}

	return renewedLease, nil
}

func (s *LeaseService) ReleaseLease(ctx context.Context, orgID, deviceID, leaseID, userID string) error {
	existing, err := s.leaseRepo.GetLease(ctx, orgID, deviceID)
	if err != nil {
		return err
	}

	if existing.ControlLeaseID != leaseID || existing.UserID != userID {
		return domain.ErrLeaseNotOwned
	}

	if err := s.leaseRepo.ReleaseLease(ctx, orgID, deviceID, leaseID, userID, existing.FencingToken); err != nil {
		return err
	}

	// Mark historical audit record revoked_at in PostgreSQL
	if err := s.fenceRepo.RevokeLeaseAudit(ctx, leaseID); err != nil {
		slog.Error("Failed to revoke control lease audit record", "err", err, "lease_id", leaseID)
		return fmt.Errorf("failed to revoke lease audit record: %w", err)
	}

	return nil
}

func (s *LeaseService) GetActiveLease(ctx context.Context, orgID, deviceID string) (*domain.ControlLease, error) {
	return s.leaseRepo.GetLease(ctx, orgID, deviceID)
}
