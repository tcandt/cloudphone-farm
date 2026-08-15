package cluster_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/internal/cluster"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	redispkg "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
)

func TestMultiNodeClusterRoutingAndFailover(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdbA := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rdbB := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	rdbC := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	defer rdbA.Close()
	defer rdbB.Close()
	defer rdbC.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Initialize Node Registries
	regA := cluster.NewNodeRegistry("node-a", rdbA, 5*time.Second)
	regB := cluster.NewNodeRegistry("node-b", rdbB, 5*time.Second)
	regC := cluster.NewNodeRegistry("node-c", rdbC, 5*time.Second)

	if err := regA.Start(ctx); err != nil {
		t.Fatalf("failed to start node A: %v", err)
	}
	if err := regB.Start(ctx); err != nil {
		t.Fatalf("failed to start node B: %v", err)
	}
	if err := regC.Start(ctx); err != nil {
		t.Fatalf("failed to start node C: %v", err)
	}

	defer regA.Shutdown(ctx)
	defer regB.Shutdown(ctx)
	defer regC.Shutdown(ctx)

	// 2. Initialize Message Buses & Hubs
	busA := cluster.NewMessageBus("node-a", rdbA)
	busB := cluster.NewMessageBus("node-b", rdbB)
	busC := cluster.NewMessageBus("node-c", rdbC)

	defer busA.Close()
	defer busB.Close()
	defer busC.Close()

	hubA := agentws.NewHub()
	hubB := agentws.NewHub()
	hubC := agentws.NewHub()

	browserHubA := agentws.NewBrowserHub()
	browserHubB := agentws.NewBrowserHub()
	browserHubC := agentws.NewBrowserHub()

	agentRepoA := redispkg.NewAgentConnectionRepository(rdbA)
	agentRepoB := redispkg.NewAgentConnectionRepository(rdbB)
	agentRepoC := redispkg.NewAgentConnectionRepository(rdbC)

	mediaRepoA := redispkg.NewMediaSessionRepository(rdbA)
	mediaRepoB := redispkg.NewMediaSessionRepository(rdbB)
	mediaRepoC := redispkg.NewMediaSessionRepository(rdbC)

	routerA := cluster.NewClusterRouter("node-a", busA, agentRepoA, mediaRepoA, hubA, browserHubA)
	routerB := cluster.NewClusterRouter("node-b", busB, agentRepoB, mediaRepoB, hubB, browserHubB)
	routerC := cluster.NewClusterRouter("node-c", busC, agentRepoC, mediaRepoC, hubC, browserHubC)

	if err := routerA.Start(ctx); err != nil {
		t.Fatalf("failed to start router A: %v", err)
	}
	if err := routerB.Start(ctx); err != nil {
		t.Fatalf("failed to start router B: %v", err)
	}
	if err := routerC.Start(ctx); err != nil {
		t.Fatalf("failed to start router C: %v", err)
	}

	time.Sleep(50 * time.Millisecond) // Allow Pub/Sub subscriptions to settle

	// 3. Register Browser Subscriber on Node B
	subB := agentws.NewBrowserSubscriber("sub_b_01", "org_test", "dev_sm_g930f_01", "user_01")
	browserHubB.Subscribe(subB)
	defer browserHubB.Unsubscribe(subB)

	// 4. Register Agent Connection on Node A (Global Gen 1)
	gen1, err := agentRepoA.NextGeneration(ctx, "org_test", "dev_sm_g930f_01")
	if err != nil || gen1 != 1 {
		t.Fatalf("expected gen1 == 1, got %d (err: %v)", gen1, err)
	}

	agentA := &domain.DeviceAgent{
		AgentID:        "agent_01",
		DeviceID:       "dev_sm_g930f_01",
		OrganizationID: "org_test",
	}
	connA := agentws.NewConnection(hubA, nil, agentA, gen1)
	connA.ConnectionID = "conn_a_01"
	hubA.Register(connA)

	ownerA := redispkg.AgentOwnerRecord{
		NodeID:       "node-a",
		AgentID:      "agent_01",
		ConnectionID: "conn_a_01",
		Generation:   gen1,
	}
	if err := agentRepoA.RegisterOwner(ctx, "org_test", "dev_sm_g930f_01", ownerA, 30*time.Second); err != nil {
		t.Fatalf("failed to register owner A: %v", err)
	}

	// 5. Verify Node C resolves owner A from Redis
	resolvedOwner, err := agentRepoC.GetOwner(ctx, "org_test", "dev_sm_g930f_01")
	if err != nil || resolvedOwner == nil || resolvedOwner.NodeID != "node-a" {
		t.Fatalf("Node C failed to resolve owner A from Redis directory: %v", err)
	}

	snap1 := agentws.ConnectionSnapshot{
		AgentID:      resolvedOwner.AgentID,
		ConnectionID: resolvedOwner.ConnectionID,
		Generation:   resolvedOwner.Generation,
	}

	// Dispatch Command Route from Node C to Node A (Must return receipt success)
	if err := routerC.DispatchCommandRoute(ctx, "org_test", "dev_sm_g930f_01", snap1, "node-a", []byte(`{"action":"home"}`)); err != nil {
		t.Fatalf("DispatchCommandRoute from Node C to Node A failed: %v", err)
	}

	// Verify Agent on Node A received dispatched message
	select {
	case msg := <-connA.Send:
		if len(msg) == 0 {
			t.Fatalf("Agent on Node A received empty payload")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("Timed out waiting for Agent on Node A to receive command")
	}

	// 6. Node A broadcasts status event -> Node B receives and forwards to browser subscriber B
	routerA.BroadcastBrowserStatusEvent("org_test", "dev_sm_g930f_01", "cmd_100", "executing", 2, "")

	select {
	case msg := <-subB.Send:
		if len(msg) == 0 {
			t.Fatalf("received empty browser message on Node B")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for cross-node browser status event on Node B")
	}

	// 7. Failover Simulation: Kill Node A, Agent reconnects to Node C (Gen 2)
	gen2, err := agentRepoC.NextGeneration(ctx, "org_test", "dev_sm_g930f_01")
	if err != nil || gen2 != 2 {
		t.Fatalf("expected gen2 == 2 after failover, got %d", gen2)
	}

	ownerC := redispkg.AgentOwnerRecord{
		NodeID:       "node-c",
		AgentID:      "agent_01",
		ConnectionID: "conn_c_02",
		Generation:   gen2,
	}
	if err := agentRepoC.RegisterOwner(ctx, "org_test", "dev_sm_g930f_01", ownerC, 30*time.Second); err != nil {
		t.Fatalf("failed to register owner C: %v", err)
	}

	// Stale owner A delayed RegisterOwner with lower gen1 MUST be rejected by Lua script
	err = agentRepoA.RegisterOwner(ctx, "org_test", "dev_sm_g930f_01", ownerA, 30*time.Second)
	if err == nil {
		t.Fatalf("expected delayed gen1 RegisterOwner to fail CAS check against gen2, got nil")
	}

	// Stale owner A renewal must fail CAS check
	err = agentRepoA.RenewOwner(ctx, "org_test", "dev_sm_g930f_01", ownerA, 30*time.Second)
	if err == nil {
		t.Fatalf("expected CAS mismatch error when stale node A tries to renew owner, got nil")
	}

	// Active owner C renewal succeeds
	err = agentRepoC.RenewOwner(ctx, "org_test", "dev_sm_g930f_01", ownerC, 30*time.Second)
	if err != nil {
		t.Fatalf("expected active owner C renewal to succeed, got %v", err)
	}
}
