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
	ErrMediaSessionNotFound = errors.New("media session not found or expired in distributed store")
)

type DistributedMediaSession struct {
	SessionID      string    `json:"session_id"`
	OrganizationID string    `json:"organization_id"`
	DeviceID       string    `json:"device_id"`
	AgentNodeID    string    `json:"agent_node_id"`
	AgentID        string    `json:"agent_id"`
	ConnectionID   string    `json:"connection_id"`
	Generation     int64     `json:"generation"`
	BrowserNodeID  string    `json:"browser_node_id"`
	SubscriberID   string    `json:"subscriber_id"`
	UserID         string    `json:"user_id"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type MediaSessionRepository struct {
	rdb *redis.Client
}

func NewMediaSessionRepository(rdb *redis.Client) *MediaSessionRepository {
	return &MediaSessionRepository{rdb: rdb}
}

func mediaSessionKey(sessionID string) string {
	return fmt.Sprintf("pcp:v1:media-session:%s", sessionID)
}

func (r *MediaSessionRepository) RegisterMediaSession(ctx context.Context, session *DistributedMediaSession, ttl time.Duration) error {
	if r.rdb == nil {
		return ErrRedisDown
	}

	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to serialize distributed media session: %w", err)
	}

	key := mediaSessionKey(session.SessionID)
	if err := r.rdb.Set(ctx, key, payload, ttl).Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrRedisDown, err)
	}

	return nil
}

func (r *MediaSessionRepository) GetMediaSession(ctx context.Context, sessionID string) (*DistributedMediaSession, error) {
	if r.rdb == nil {
		return nil, ErrRedisDown
	}

	key := mediaSessionKey(sessionID)
	val, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrMediaSessionNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrRedisDown, err)
	}

	var session DistributedMediaSession
	if err := json.Unmarshal([]byte(val), &session); err != nil {
		return nil, fmt.Errorf("failed to deserialize distributed media session: %w", err)
	}

	return &session, nil
}

func (r *MediaSessionRepository) UnregisterMediaSession(ctx context.Context, sessionID string) error {
	if r.rdb == nil {
		return ErrRedisDown
	}

	key := mediaSessionKey(sessionID)
	if err := r.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrRedisDown, err)
	}

	return nil
}
