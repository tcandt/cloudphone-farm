package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrOwnerNotFound = errors.New("agent owner record not found")
	ErrCASMismatch   = errors.New("agent owner CAS mismatch: lease expired or owned by another node/generation")
)

type AgentOwnerRecord struct {
	NodeID       string `json:"node_id"`
	AgentID      string `json:"agent_id"`
	ConnectionID string `json:"connection_id"`
	Generation   int64  `json:"generation"`
	UpdatedAt    string `json:"updated_at"`
}

type AgentConnectionRepository struct {
	rdb *redis.Client
}

func NewAgentConnectionRepository(rdb *redis.Client) *AgentConnectionRepository {
	return &AgentConnectionRepository{rdb: rdb}
}

func generationKey(orgID, deviceID string) string {
	return fmt.Sprintf("pcp:v1:agent-generation:%s:%s", orgID, deviceID)
}

func ownerKey(orgID, deviceID string) string {
	return fmt.Sprintf("pcp:v1:agent-owner:%s:%s", orgID, deviceID)
}

// NextGeneration increments the globally monotonic generation counter in Redis (NO TTL).
// Fail closed if Redis is down or unavailable.
func (r *AgentConnectionRepository) NextGeneration(ctx context.Context, orgID, deviceID string) (int64, error) {
	if r.rdb == nil {
		return 0, ErrRedisDown
	}

	key := generationKey(orgID, deviceID)
	gen, err := r.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("%w: failed to increment generation counter: %v", ErrRedisDown, err)
	}

	return gen, nil
}

// GetGeneration retrieves current global generation count without incrementing.
func (r *AgentConnectionRepository) GetGeneration(ctx context.Context, orgID, deviceID string) (int64, error) {
	if r.rdb == nil {
		return 0, ErrRedisDown
	}

	key := generationKey(orgID, deviceID)
	val, err := r.rdb.Get(ctx, key).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, fmt.Errorf("%w: %v", ErrRedisDown, err)
	}

	return val, nil
}

// RegisterOwner uses a Lua script to atomically claim owner status ONLY IF proposed generation matches current global generation and >= existing owner generation.
const registerOwnerScript = `
local owner_key = KEYS[1]
local gen_key = KEYS[2]
local expected_gen = tonumber(ARGV[1])
local new_payload = ARGV[2]
local ttl_ms = tonumber(ARGV[3])

local current_gen_str = redis.call('GET', gen_key)
local current_gen = current_gen_str and tonumber(current_gen_str) or 0

if expected_gen < current_gen then
    return 0
end

local existing_owner = redis.call('GET', owner_key)
if existing_owner then
    local data = cjson.decode(existing_owner)
    if data and data.generation and tonumber(data.generation) > expected_gen then
        return 0
    end
end

redis.call('SET', owner_key, new_payload, 'PX', ttl_ms)
return 1
`

func (r *AgentConnectionRepository) RegisterOwner(ctx context.Context, orgID, deviceID string, rec AgentOwnerRecord, ttl time.Duration) error {
	if r.rdb == nil {
		return ErrRedisDown
	}

	rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	payload, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("failed to serialize agent owner record: %w", err)
	}

	oKey := ownerKey(orgID, deviceID)
	gKey := generationKey(orgID, deviceID)
	ttlMs := ttl.Milliseconds()

	res, err := r.rdb.Eval(ctx, registerOwnerScript, []string{oKey, gKey}, rec.Generation, string(payload), ttlMs).Int64()
	if err != nil {
		return fmt.Errorf("%w: atomic register owner script error: %v", ErrRedisDown, err)
	}

	if res == 0 {
		return ErrCASMismatch
	}

	return nil
}

// RenewOwner uses a CAS Lua script to renew TTL ONLY IF the current owner record matches connection_id AND generation.
const renewOwnerScript = `
local key = KEYS[1]
local expected_conn_id = ARGV[1]
local expected_gen = tonumber(ARGV[2])
local new_payload = ARGV[3]
local ttl_ms = tonumber(ARGV[4])

local current = redis.call('GET', key)
if not current then
    return 0
end

local data = cjson.decode(current)
if data.connection_id == expected_conn_id and tonumber(data.generation) == expected_gen then
    redis.call('SET', key, new_payload, 'PX', ttl_ms)
    return 1
else
    return 0
end
`

func (r *AgentConnectionRepository) RenewOwner(ctx context.Context, orgID, deviceID string, rec AgentOwnerRecord, ttl time.Duration) error {
	if r.rdb == nil {
		return ErrRedisDown
	}

	rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	payload, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("failed to serialize agent owner record: %w", err)
	}

	key := ownerKey(orgID, deviceID)
	ttlMs := ttl.Milliseconds()

	res, err := r.rdb.Eval(ctx, renewOwnerScript, []string{key}, rec.ConnectionID, rec.Generation, string(payload), ttlMs).Int64()
	if err != nil {
		return fmt.Errorf("%w: CAS renew script error: %v", ErrRedisDown, err)
	}

	if res == 0 {
		return ErrCASMismatch
	}

	return nil
}

// UnregisterOwner uses a CAS Lua script to delete the owner record ONLY IF the current record matches connection_id AND generation.
const unregisterOwnerScript = `
local key = KEYS[1]
local expected_conn_id = ARGV[1]
local expected_gen = tonumber(ARGV[2])

local current = redis.call('GET', key)
if not current then
    return 1
end

local data = cjson.decode(current)
if data.connection_id == expected_conn_id and tonumber(data.generation) == expected_gen then
    redis.call('DEL', key)
    return 1
else
    return 0
end
`

func (r *AgentConnectionRepository) UnregisterOwner(ctx context.Context, orgID, deviceID string, connID string, generation int64) error {
	if r.rdb == nil {
		return ErrRedisDown
	}

	key := ownerKey(orgID, deviceID)
	res, err := r.rdb.Eval(ctx, unregisterOwnerScript, []string{key}, connID, generation).Int64()
	if err != nil {
		return fmt.Errorf("%w: CAS unregister script error: %v", ErrRedisDown, err)
	}

	if res == 0 {
		return ErrCASMismatch
	}

	return nil
}

// GetOwner fetches the current active agent owner record from Redis.
func (r *AgentConnectionRepository) GetOwner(ctx context.Context, orgID, deviceID string) (*AgentOwnerRecord, error) {
	if r.rdb == nil {
		return nil, ErrRedisDown
	}

	key := ownerKey(orgID, deviceID)
	val, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrOwnerNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrRedisDown, err)
	}

	var rec AgentOwnerRecord
	if err := json.Unmarshal([]byte(val), &rec); err != nil {
		return nil, fmt.Errorf("failed to deserialize agent owner record: %w", err)
	}

	return &rec, nil
}
