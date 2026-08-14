package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

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

// UpdatePresence stores or refreshes realtime presence in Redis with a 30s TTL
func (r *PresenceRepository) UpdatePresence(ctx context.Context, orgID, deviceID string, p *domain.AgentPresencePayload) error {
	if r.rdb == nil {
		return ErrRedisDown
	}

	key := presenceKey(orgID, deviceID)

	// Fetch existing presence to check generation / sequence stale guard
	val, err := r.rdb.Get(ctx, key).Result()
	if err == nil && val != "" {
		var existing domain.AgentPresencePayload
		if err := json.Unmarshal([]byte(val), &existing); err == nil {
			// Ignore stale sequence if generation is identical but sequence is smaller
			if existing.Generation == p.Generation && p.Sequence < existing.Sequence {
				return nil
			}
			if p.Generation < existing.Generation {
				return nil
			}
		}
	}

	data, err := json.Marshal(p)
	if err != nil {
		return fmt.Errorf("failed to serialize presence payload: %w", err)
	}

	if err := r.rdb.Set(ctx, key, data, r.ttl).Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrRedisDown, err)
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
