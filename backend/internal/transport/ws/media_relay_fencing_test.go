package ws_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/internal/cluster"
	redispkg "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
	wstransport "github.com/tcandt/cloudphone-farm/backend/internal/transport/ws"
)

func TestMediaRelayRejectsStaleGlobalOwner(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	ctx := context.Background()
	agentRepo := redispkg.NewAgentConnectionRepository(rdb)
	mediaRepo := redispkg.NewMediaSessionRepository(rdb)

	busA := cluster.NewMessageBus("node-a", rdb)
	defer busA.Close()

	hubA := agentws.NewHub()
	browserHubA := agentws.NewBrowserHub()
	routerA := cluster.NewClusterRouter("node-a", busA, agentRepo, mediaRepo, hubA, browserHubA)

	relayer := wstransport.NewClusterMediaRelayer(mediaRepo, agentRepo, routerA)

	orgID := "org_test"
	deviceID := "dev_fenced_01"

	// 1. Register owner gen1 on Node A
	ownerGen1 := redispkg.AgentOwnerRecord{
		NodeID:       "node-a",
		AgentID:      "agent_01",
		ConnectionID: "conn_gen1",
		Generation:   1,
	}
	_, _ = agentRepo.NextGeneration(ctx, orgID, deviceID)
	if err := agentRepo.RegisterOwner(ctx, orgID, deviceID, ownerGen1, 30*time.Second); err != nil {
		t.Fatalf("failed to register owner gen1: %v", err)
	}

	// Register Media Session for gen1
	distSession := &redispkg.DistributedMediaSession{
		SessionID:      "sess_media_100",
		OrganizationID: orgID,
		DeviceID:       deviceID,
		AgentID:        "agent_01",
		ConnectionID:   "conn_gen1",
		Generation:     1,
		BrowserNodeID:  "node-b",
		SubscriberID:   "sub_b",
		UserID:         "user_01",
		CreatedAt:      time.Now().UTC(),
		ExpiresAt:      time.Now().Add(15 * time.Minute).UTC(),
	}
	if err := mediaRepo.RegisterMediaSession(ctx, distSession, 15*time.Minute); err != nil {
		t.Fatalf("failed to register media session: %v", err)
	}

	// 2. Failover: gen2 takes over as active owner in Redis
	_, _ = agentRepo.NextGeneration(ctx, orgID, deviceID)
	ownerGen2 := redispkg.AgentOwnerRecord{
		NodeID:       "node-c",
		AgentID:      "agent_01",
		ConnectionID: "conn_gen2",
		Generation:   2,
	}
	if err := agentRepo.RegisterOwner(ctx, orgID, deviceID, ownerGen2, 30*time.Second); err != nil {
		t.Fatalf("failed to register owner gen2: %v", err)
	}

	// 3. Stale gen1 Agent attempts media relay post-takeover -> MUST BE REJECTED IMMEDIATELY
	staleConn := &agentws.Connection{
		OrganizationID: orgID,
		DeviceID:       deviceID,
		AgentID:        "agent_01",
		ConnectionID:   "conn_gen1",
		Generation:     1,
	}

	err = relayer.RelayMediaSignalToBrowser(ctx, staleConn, "sess_media_100", []byte(`{"type":"answer"}`))
	if err != agentws.ErrUnauthorizedMediaSession {
		t.Fatalf("expected ErrUnauthorizedMediaSession when stale gen1 connection relays media after gen2 takeover, got: %v", err)
	}
}
