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
	ErrSessionNotFound = errors.New("session not found or expired")
	ErrRedisDown        = errors.New("authoritative redis session store unavailable")
)

type SessionData struct {
	SessionID      string              `json:"session_id"`
	UserID         string              `json:"user_id"`
	Email          string              `json:"email"`
	DisplayName    string              `json:"display_name"`
	OrganizationID string              `json:"organization_id"`
	MembershipID   string              `json:"membership_id"`
	Roles          []string            `json:"roles"`
	Permissions    map[string]struct{} `json:"permissions"`
}

type SessionRepository struct {
	rdb *redis.Client
}

func NewSessionRepository(rdb *redis.Client) *SessionRepository {
	return &SessionRepository{
		rdb: rdb,
	}
}

func sessionKey(tokenHash string) string {
	return fmt.Sprintf("pcp:session:v1:%s", tokenHash)
}

// CreateSession stores an authoritative session in Redis by SHA-256 token hash (never raw token!)
func (r *SessionRepository) CreateSession(ctx context.Context, tokenHash string, data *SessionData, ttl time.Duration) error {
	if r.rdb == nil {
		return ErrRedisDown
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to serialize session data: %w", err)
	}

	key := sessionKey(tokenHash)
	if err := r.rdb.Set(ctx, key, payload, ttl).Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrRedisDown, err)
	}

	return nil
}

// GetSession retrieves the authoritative session from Redis by SHA-256 token hash
func (r *SessionRepository) GetSession(ctx context.Context, tokenHash string) (*SessionData, error) {
	if r.rdb == nil {
		return nil, ErrRedisDown
	}

	key := sessionKey(tokenHash)
	val, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrRedisDown, err)
	}

	var data SessionData
	if err := json.Unmarshal([]byte(val), &data); err != nil {
		return nil, fmt.Errorf("failed to deserialize session data: %w", err)
	}

	return &data, nil
}

// DeleteSession invalidates an authoritative session in Redis by SHA-256 token hash
func (r *SessionRepository) DeleteSession(ctx context.Context, tokenHash string) error {
	if r.rdb == nil {
		return ErrRedisDown
	}

	key := sessionKey(tokenHash)
	if err := r.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrRedisDown, err)
	}

	return nil
}
