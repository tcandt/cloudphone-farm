package agentenrollment

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	ChallengeTTL = 120 * time.Second
)

type ChallengeContext struct {
	KeyID                string `json:"key_id"`
	OrganizationID       string `json:"organization_id"`
	ClientInstanceID     string `json:"client_instance_id"`
	PublicKeyFingerprint string `json:"public_key_fingerprint"`
	Nonce                string `json:"nonce"` // base64 encoded
}

type ChallengeStore struct {
	rdb *redis.Client
}

func NewChallengeStore(rdb *redis.Client) *ChallengeStore {
	return &ChallengeStore{
		rdb: rdb,
	}
}

func (s *ChallengeStore) SaveChallenge(ctx context.Context, challengeID string, challengeCtx ChallengeContext) error {
	key := fmt.Sprintf("agent:enroll:challenge:%s", challengeID)
	data, err := json.Marshal(challengeCtx)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, key, data, ChallengeTTL).Err()
}

func (s *ChallengeStore) ConsumeChallenge(ctx context.Context, challengeID string) (*ChallengeContext, error) {
	key := fmt.Sprintf("agent:enroll:challenge:%s", challengeID)

	// Atomically get and delete the challenge
	data, err := s.rdb.GetDel(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, fmt.Errorf("challenge not found or expired")
		}
		return nil, err
	}

	var challengeCtx ChallengeContext
	if err := json.Unmarshal([]byte(data), &challengeCtx); err != nil {
		return nil, fmt.Errorf("failed to parse challenge context: %w", err)
	}

	return &challengeCtx, nil
}
