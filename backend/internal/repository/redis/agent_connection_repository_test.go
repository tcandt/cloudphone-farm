package redis_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	redispkg "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
)

func TestRegisterOwner_FutureAndOldGenerationRejection(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	repo := redispkg.NewAgentConnectionRepository(rdb)
	ctx := context.Background()

	orgID := "org_test"
	deviceID := "dev_01"

	// 1. INCR current generation to 42
	for i := 0; i < 42; i++ {
		_, err := repo.NextGeneration(ctx, orgID, deviceID)
		if err != nil {
			t.Fatalf("failed to increment generation: %v", err)
		}
	}

	currGen, err := repo.GetGeneration(ctx, orgID, deviceID)
	if err != nil || currGen != 42 {
		t.Fatalf("expected current generation 42, got %d (err: %v)", currGen, err)
	}

	// 2. Test OLD generation claim (gen = 41) MUST BE REJECTED
	oldOwner := redispkg.AgentOwnerRecord{
		NodeID:       "node-a",
		AgentID:      "agent_01",
		ConnectionID: "conn_old",
		Generation:   41,
	}
	err = repo.RegisterOwner(ctx, orgID, deviceID, oldOwner, 30*time.Second)
	if err == nil {
		t.Fatalf("expected old generation 41 claim to be rejected when current = 42, got nil error")
	}

	// 3. Test FUTURE generation claim (gen = 43) MUST BE REJECTED (expected_gen ~= current_gen rule)
	futureOwner := redispkg.AgentOwnerRecord{
		NodeID:       "node-b",
		AgentID:      "agent_01",
		ConnectionID: "conn_future",
		Generation:   43,
	}
	err = repo.RegisterOwner(ctx, orgID, deviceID, futureOwner, 30*time.Second)
	if err == nil {
		t.Fatalf("expected future generation 43 claim to be rejected when current = 42, got nil error")
	}

	// 4. Test EXACT generation claim (gen = 42) MUST SUCCEED
	exactOwner := redispkg.AgentOwnerRecord{
		NodeID:       "node-c",
		AgentID:      "agent_01",
		ConnectionID: "conn_exact",
		Generation:   42,
	}
	err = repo.RegisterOwner(ctx, orgID, deviceID, exactOwner, 30*time.Second)
	if err != nil {
		t.Fatalf("expected exact generation 42 claim to succeed, got: %v", err)
	}
}
