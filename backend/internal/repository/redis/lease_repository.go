package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

type LeaseRepository struct {
	rdb *redis.Client
}

func NewLeaseRepository(rdb *redis.Client) *LeaseRepository {
	return &LeaseRepository{rdb: rdb}
}

func leaseKey(orgID, deviceID string) string {
	return fmt.Sprintf("pcp:control:lease:v1:%s:%s", orgID, deviceID)
}

// AcquireLease attempts to acquire exclusive 30s control lease in Redis
func (r *LeaseRepository) AcquireLease(ctx context.Context, lease *domain.ControlLease) error {
	if r.rdb == nil {
		return fmt.Errorf("redis client uninitialized")
	}

	key := leaseKey(lease.OrganizationID, lease.DeviceID)
	data, err := json.Marshal(lease)
	if err != nil {
		return fmt.Errorf("failed to marshal lease: %w", err)
	}

	ok, err := r.rdb.SetNX(ctx, key, data, 30*time.Second).Result()
	if err != nil {
		return fmt.Errorf("failed to acquire lease in redis: %w", err)
	}
	if !ok {
		return domain.ErrControlAlreadyLeased
	}

	return nil
}

// GetLease fetches active control lease from Redis
func (r *LeaseRepository) GetLease(ctx context.Context, orgID, deviceID string) (*domain.ControlLease, error) {
	if r.rdb == nil {
		return nil, fmt.Errorf("redis client uninitialized")
	}

	key := leaseKey(orgID, deviceID)
	val, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, domain.ErrLeaseNotFound
		}
		return nil, fmt.Errorf("failed to fetch lease from redis: %w", err)
	}

	var lease domain.ControlLease
	if err := json.Unmarshal([]byte(val), &lease); err != nil {
		return nil, fmt.Errorf("failed to unmarshal lease: %w", err)
	}

	return &lease, nil
}

// RenewLease uses Redis CAS Lua script to renew lease TTL only if lease_id, user_id, and fencing_token match
func (r *LeaseRepository) RenewLease(ctx context.Context, orgID, deviceID, leaseID, userID string, fencingToken int64) (*domain.ControlLease, error) {
	if r.rdb == nil {
		return nil, fmt.Errorf("redis client uninitialized")
	}

	key := leaseKey(orgID, deviceID)
	script := redis.NewScript(`
		local val = redis.call('GET', KEYS[1])
		if not val then return nil end
		local data = cjson.decode(val)
		if data.control_lease_id == ARGV[1] and data.user_id == ARGV[2] and tonumber(data.fencing_token) == tonumber(ARGV[3]) then
			data.expires_at = ARGV[4]
			data.ttl_seconds = 30
			local updated = cjson.encode(data)
			redis.call('SET', KEYS[1], updated, 'EX', 30)
			return updated
		end
		return nil
	`)

	newExpiresAt := time.Now().Add(30 * time.Second).UTC().Format(time.RFC3339)
	res, err := script.Run(ctx, r.rdb, []string{key}, leaseID, userID, fencingToken, newExpiresAt).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, domain.ErrLeaseNotOwned
		}
		return nil, fmt.Errorf("failed to renew lease in redis: %w", err)
	}

	valStr, ok := res.(string)
	if !ok || valStr == "" {
		return nil, domain.ErrLeaseNotOwned
	}

	var lease domain.ControlLease
	if err := json.Unmarshal([]byte(valStr), &lease); err != nil {
		return nil, fmt.Errorf("failed to unmarshal renewed lease: %w", err)
	}

	return &lease, nil
}

// ReleaseLease uses Redis CAS Lua script to release lease only if lease_id, user_id, and fencing_token match
func (r *LeaseRepository) ReleaseLease(ctx context.Context, orgID, deviceID, leaseID, userID string, fencingToken int64) error {
	if r.rdb == nil {
		return fmt.Errorf("redis client uninitialized")
	}

	key := leaseKey(orgID, deviceID)
	script := redis.NewScript(`
		local val = redis.call('GET', KEYS[1])
		if not val then return 0 end
		local data = cjson.decode(val)
		if data.control_lease_id == ARGV[1] and data.user_id == ARGV[2] and tonumber(data.fencing_token) == tonumber(ARGV[3]) then
			redis.call('DEL', KEYS[1])
			return 1
		end
		return 0
	`)

	res, err := script.Run(ctx, r.rdb, []string{key}, leaseID, userID, fencingToken).Result()
	if err != nil {
		return fmt.Errorf("failed to release lease in redis: %w", err)
	}

	affected, _ := res.(int64)
	if affected == 0 {
		return domain.ErrLeaseNotOwned
	}

	return nil
}
