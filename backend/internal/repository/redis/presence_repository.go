package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

var atomicPresenceLua = redis.NewScript(`
local key = KEYS[1]
local payload = ARGV[1]
local ttl = tonumber(ARGV[2])
local new_gen = tonumber(ARGV[3])
local new_seq = tonumber(ARGV[4])

local existing = redis.call('GET', key)
if existing and existing ~= '' then
    local ok, data = pcall(cjson.decode, existing)
    if ok and data then
        local old_gen = tonumber(data['generation'] or 0)
        local old_seq = tonumber(data['sequence'] or 0)

        if old_gen > new_gen then
            return 0
        end
        if old_gen == new_gen and old_seq > new_seq then
            return 0
        end
    end
end

redis.call('SET', key, payload, 'EX', ttl)
return 1
`)

type PresenceRepository struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewPresenceRepository(rdb *redis.Client, ttl time.Duration) *PresenceRepository {
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &PresenceRepository{
		rdb: rdb,
		ttl: ttl,
	}
}

func presenceKey(orgID, deviceID string) string {
	return fmt.Sprintf("pcp:presence:v1:%s:%s", orgID, deviceID)
}

// UpdatePresence executes atomic Lua CAS presence update in Redis with a 30s TTL
func (r *PresenceRepository) UpdatePresence(ctx context.Context, orgID, deviceID string, p *domain.AgentPresencePayload) error {
	if r.rdb == nil {
		return ErrRedisDown
	}

	key := presenceKey(orgID, deviceID)
	wasOnline := r.rdb.Exists(ctx, key).Val() > 0

	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("failed to serialize presence payload: %w", err)
	}

	ttlSec := int(r.ttl.Seconds())
	keys := []string{key}
	args := []interface{}{string(data), ttlSec, p.Generation, p.Sequence}

	if err := atomicPresenceLua.Run(ctx, r.rdb, keys, args...).Err(); err != nil {
		return fmt.Errorf("%w: atomic presence Lua execution failed: %v", ErrRedisDown, err)
	}

	if !wasOnline {
		_ = r.PublishPresenceTransition(ctx, orgID, deviceID, "agent_online")
	}

	return nil
}

// GetPresence retrieves current presence status from Redis
func (r *PresenceRepository) GetPresence(ctx context.Context, orgID, deviceID string) (*domain.AgentPresencePayload, error) {
	if r.rdb == nil {
		return nil, ErrRedisDown
	}

	key := presenceKey(orgID, deviceID)
	val, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrRedisDown, err)
	}

	var p domain.AgentPresencePayload
	if err := json.Unmarshal([]byte(val), &p); err != nil {
		return nil, fmt.Errorf("failed to deserialize presence payload: %w", err)
	}

	return &p, nil
}

var atomicRemovePresenceLua = redis.NewScript(`
local key = KEYS[1]
local req_gen = tonumber(ARGV[1] or 0)

local existing = redis.call('GET', key)
if existing and existing ~= '' then
    local ok, data = pcall(cjson.decode, existing)
    if ok and data then
        local current_gen = tonumber(data['generation'] or 0)
        if req_gen > 0 and current_gen > req_gen then
            return 0
        end
    end
    redis.call('DEL', key)
    return 1
end
redis.call('DEL', key)
return 1
`)

// RemovePresence deletes presence key from Redis upon Agent disconnect or revocation with optional generation fencing
func (r *PresenceRepository) RemovePresence(ctx context.Context, orgID, deviceID string) error {
	return r.RemovePresenceFenced(ctx, orgID, deviceID, 0)
}

func (r *PresenceRepository) RemovePresenceFenced(ctx context.Context, orgID, deviceID string, reqGen int64) error {
	if r.rdb == nil {
		return ErrRedisDown
	}

	key := presenceKey(orgID, deviceID)
	keys := []string{key}
	args := []interface{}{reqGen}

	res, err := atomicRemovePresenceLua.Run(ctx, r.rdb, keys, args...).Int64()
	if err != nil {
		return fmt.Errorf("%w: atomic remove presence Lua failed: %v", ErrRedisDown, err)
	}

	if res > 0 {
		_ = r.PublishPresenceTransition(ctx, orgID, deviceID, "agent_offline")
	}
	return nil
}

// PublishPresenceTransition publishes combined state change event agent_online / agent_offline
func (r *PresenceRepository) PublishPresenceTransition(ctx context.Context, orgID, deviceID, eventType string) error {
	if r.rdb == nil {
		return nil
	}
	ch := fmt.Sprintf("pcp:v1:device-bus:%s", deviceID)
	evt := map[string]interface{}{
		"message_id":      fmt.Sprintf("pres_%d", time.Now().UnixNano()),
		"source_node_id":  "presence_repository",
		"target_node_id":  "",
		"organization_id": orgID,
		"device_id":       deviceID,
		"type":           eventType,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}
	bytes, _ := json.Marshal(evt)
	return r.rdb.Publish(ctx, ch, bytes).Err()
}
