package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const MaxDirectP2PViewersPerDevice = 1

var (
	ErrViewerQuotaExceeded = errors.New("maximum direct P2P viewer stream quota exceeded for device (max 1)")
)

type ViewerRepository struct {
	rdb *redis.Client
}

func NewViewerRepository(rdb *redis.Client) *ViewerRepository {
	return &ViewerRepository{rdb: rdb}
}

func viewerQuotaKey(orgID, deviceID string) string {
	return fmt.Sprintf("pcp:v1:viewers:%s:%s", orgID, deviceID)
}

const acquireViewerScript = `
local key = KEYS[1]
local viewer_id = ARGV[1]
local max_viewers = tonumber(ARGV[2])
local now_ms = tonumber(ARGV[3])
local ttl_ms = tonumber(ARGV[4])

-- Clean up expired viewer leases
redis.call('ZREMRANGEBYSCORE', key, '-inf', now_ms)

-- Check current active viewer count
local count = redis.call('ZCARD', key)

-- Check if this viewer already holds an active lease
local existing_score = redis.call('ZSCORE', key, viewer_id)

if existing_score or count < max_viewers then
    local expires_at = now_ms + ttl_ms
    redis.call('ZADD', key, expires_at, viewer_id)
    redis.call('PEXPIRE', key, ttl_ms * 2)
    return 1
else
    return 0
end
`

const releaseViewerScript = `
local key = KEYS[1]
local viewer_id = ARGV[1]
redis.call('ZREM', key, viewer_id)
return 1
`

func (r *ViewerRepository) AcquireViewerLease(ctx context.Context, orgID, deviceID, viewerID string, ttl time.Duration) error {
	if r.rdb == nil {
		return ErrRedisDown
	}

	key := viewerQuotaKey(orgID, deviceID)
	nowMs := time.Now().UnixNano() / int64(time.Millisecond)
	ttlMs := ttl.Milliseconds()

	res, err := r.rdb.Eval(ctx, acquireViewerScript, []string{key}, viewerID, MaxDirectP2PViewersPerDevice, nowMs, ttlMs).Int64()
	if err != nil {
		return fmt.Errorf("%w: acquire viewer script error: %v", ErrRedisDown, err)
	}

	if res == 0 {
		return ErrViewerQuotaExceeded
	}

	return nil
}

func (r *ViewerRepository) ReleaseViewerLease(ctx context.Context, orgID, deviceID, viewerID string) error {
	if r.rdb == nil {
		return ErrRedisDown
	}

	key := viewerQuotaKey(orgID, deviceID)
	if err := r.rdb.Eval(ctx, releaseViewerScript, []string{key}, viewerID).Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrRedisDown, err)
	}

	return nil
}

func (r *ViewerRepository) GetActiveViewerCount(ctx context.Context, orgID, deviceID string) (int, error) {
	if r.rdb == nil {
		return 0, ErrRedisDown
	}

	key := viewerQuotaKey(orgID, deviceID)
	nowMs := time.Now().UnixNano() / int64(time.Millisecond)
	_ = r.rdb.ZRemRangeByScore(ctx, key, "-inf", fmt.Sprintf("%d", nowMs)).Err()

	count, err := r.rdb.ZCard(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrRedisDown, err)
	}

	return int(count), nil
}
