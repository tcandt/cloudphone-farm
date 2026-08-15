package cluster_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	"github.com/tcandt/cloudphone-farm/backend/internal/cluster"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	redispkg "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
	httptransport "github.com/tcandt/cloudphone-farm/backend/internal/transport/http"
	custommw "github.com/tcandt/cloudphone-farm/backend/internal/transport/http/middleware"
	wstransport "github.com/tcandt/cloudphone-farm/backend/internal/transport/ws"
)

func performAgentHandshake(conn *websocket.Conn) error {
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msgData, err := conn.ReadMessage()
	if err != nil {
		return err
	}

	var env agentws.WSEnvelope
	if err := json.Unmarshal(msgData, &env); err != nil {
		return err
	}

	var chalPayload agentws.ServerChallengePayload
	_ = json.Unmarshal(env.Payload, &chalPayload)

	respPayload := agentws.AgentChallengeResponsePayload{
		ChallengeSignature: "dummy_signature_for_test",
	}
	respBytes, _ := json.Marshal(respPayload)
	respEnv := agentws.WSEnvelope{
		Type:      agentws.MessageTypeAgentChallengeResponse,
		MessageID: "resp_01",
		Payload:   respBytes,
	}
	respEnvBytes, _ := json.Marshal(respEnv)

	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	return conn.WriteMessage(websocket.TextMessage, respEnvBytes)
}

func TestThreeNodeClusterE2EAndFailover(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	agentRepo := redispkg.NewAgentConnectionRepository(rdb)
	mediaRepo := redispkg.NewMediaSessionRepository(rdb)

	orgID := "org_acme_e2e"
	userID := "user_operator_01"
	deviceID := "dev_samsung_s24"
	agentID := "agent_node_01"

	agentObj := &domain.DeviceAgent{
		AgentID:        agentID,
		OrganizationID: orgID,
		DeviceID:       deviceID,
		Status:         "active",
		PublicKey:      nil,
	}

	// Node A: Agent WebSocket Hub & Cluster Router
	busA := cluster.NewMessageBus("node-a", rdb)
	defer busA.Close()
	hubA := agentws.NewHub()
	browserHubA := agentws.NewBrowserHub()
	routerA := cluster.NewClusterRouter("node-a", busA, agentRepo, mediaRepo, hubA, browserHubA)
	if err := routerA.Start(ctx); err != nil {
		t.Fatalf("failed to start routerA: %v", err)
	}

	agentWSHandlerA := wstransport.NewAgentWSHandler(hubA, nil, nil, browserHubA)
	agentWSHandlerA.SetClusterComponents("node-a", agentRepo, routerA)

	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqWithCtx := r.WithContext(context.WithValue(r.Context(), custommw.AgentContextKey, agentObj))
		agentWSHandlerA.Connect(w, reqWithCtx)
	}))
	defer serverA.Close()

	// Node B: Browser Event WebSocket Hub & Cluster Router
	busB := cluster.NewMessageBus("node-b", rdb)
	defer busB.Close()
	hubB := agentws.NewHub()
	browserHubB := agentws.NewBrowserHub()
	routerB := cluster.NewClusterRouter("node-b", busB, agentRepo, mediaRepo, hubB, browserHubB)
	if err := routerB.Start(ctx); err != nil {
		t.Fatalf("failed to start routerB: %v", err)
	}

	browserWSHandlerB := httptransport.NewBrowserWSHandler(browserHubB, nil, []string{"*"})

	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Origin", "http://localhost:3000")
		ctxWithPrincipal := auth.WithPrincipal(r.Context(), &auth.Principal{
			UserID:         userID,
			OrganizationID: orgID,
		})
		reqWithCtx := r.WithContext(ctxWithPrincipal)
		browserWSHandlerB.ServeHTTP(w, reqWithCtx)
	}))
	defer serverB.Close()

	// Node C: Cluster Router C
	busC := cluster.NewMessageBus("node-c", rdb)
	defer busC.Close()
	hubC := agentws.NewHub()
	browserHubC := agentws.NewBrowserHub()
	routerC := cluster.NewClusterRouter("node-c", busC, agentRepo, mediaRepo, hubC, browserHubC)
	if err := routerC.Start(ctx); err != nil {
		t.Fatalf("failed to start routerC: %v", err)
	}

	// 1. Connect synthetic Agent WS -> Node A & Perform Challenge Handshake
	wsURLA := "ws" + strings.TrimPrefix(serverA.URL, "http")
	agentWSConnA, _, err := websocket.DefaultDialer.Dial(wsURLA, nil)
	if err != nil {
		t.Fatalf("failed to connect Agent WS to Node A: %v", err)
	}
	defer agentWSConnA.Close()

	if err := performAgentHandshake(agentWSConnA); err != nil {
		t.Fatalf("failed agent challenge handshake on Node A: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	owner, err := agentRepo.GetOwner(ctx, orgID, deviceID)
	if err != nil || owner == nil {
		t.Fatalf("expected Agent owner in Redis on Node A, got err=%v owner=%v", err, owner)
	}
	if owner.NodeID != "node-a" || owner.Generation != 1 {
		t.Fatalf("expected Node A owner gen 1, got node=%s gen=%d", owner.NodeID, owner.Generation)
	}

	// 2. Connect Browser WS -> Node B
	wsURLB := "ws" + strings.TrimPrefix(serverB.URL, "http")
	browserWSConnB, _, err := websocket.DefaultDialer.Dial(wsURLB, nil)
	if err != nil {
		t.Fatalf("failed to connect Browser WS to Node B: %v", err)
	}
	defer browserWSConnB.Close()

	// 3. Direct Cross-Node Dispatch Command from Node C to Node A via Router C
	snap1 := agentws.ConnectionSnapshot{
		AgentID:      owner.AgentID,
		ConnectionID: owner.ConnectionID,
		Generation:   owner.Generation,
	}

	cmdID1 := "cmd_e2e_1001"
	cmdEnvelope := map[string]interface{}{
		"command_id":      cmdID1,
		"type":            "input.tap",
		"params":          map[string]interface{}{"x": 500, "y": 1000},
		"attempt_number":  1,
		"status":          "prepared",
		"expires_at_unix": time.Now().Add(5 * time.Minute).Unix(),
	}
	cmdBytes, _ := json.Marshal(cmdEnvelope)

	err = routerC.DispatchCommandRoute(ctx, orgID, deviceID, snap1, owner.NodeID, cmdBytes)
	if err != nil {
		t.Fatalf("expected routerC.DispatchCommandRoute to succeed, got: %v", err)
	}

	// Agent WS on Node A receives command message!
	_ = agentWSConnA.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, rMsgBytesA, err := agentWSConnA.ReadMessage()
	if err != nil {
		t.Fatalf("expected Agent WS on Node A to receive command payload, got: %v", err)
	}

	var recCmd map[string]interface{}
	_ = json.Unmarshal(rMsgBytesA, &recCmd)
	if recCmd["command_id"] != cmdID1 {
		t.Fatalf("expected command_id %s on Agent socket, got: %v", cmdID1, recCmd)
	}

	// 4. Node A broadcasts status event to Redis bus -> Node B receives and delivers to Browser B WS
	routerA.BroadcastBrowserStatusEvent(orgID, deviceID, cmdID1, "ack", 1, "")

	// Read event on Browser B socket
	_ = browserWSConnB.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, bMsgBytes, err := browserWSConnB.ReadMessage()
	if err != nil {
		t.Fatalf("expected Browser WS on Node B to receive status event, got err: %v", err)
	}

	var bEvent map[string]interface{}
	_ = json.Unmarshal(bMsgBytes, &bEvent)
	if bEvent["type"] != "command.status.changed" {
		t.Fatalf("expected type command.status.changed in Browser event on Node B, got: %v", bEvent)
	}

	// 5. LIVE FAILOVER: Close Agent WS on Node A (Node A crash simulation)
	agentWSConnA.Close()
	time.Sleep(100 * time.Millisecond)

	// Reconnect Agent WS -> Node C (Node C becomes active owner with Generation 2)
	agentWSHandlerC := wstransport.NewAgentWSHandler(hubC, nil, nil, browserHubC)
	agentWSHandlerC.SetClusterComponents("node-c", agentRepo, routerC)

	serverAgentC := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqWithCtx := r.WithContext(context.WithValue(r.Context(), custommw.AgentContextKey, agentObj))
		agentWSHandlerC.Connect(w, reqWithCtx)
	}))
	defer serverAgentC.Close()

	wsURLAgentC := "ws" + strings.TrimPrefix(serverAgentC.URL, "http")
	agentWSConnC, _, err := websocket.DefaultDialer.Dial(wsURLAgentC, nil)
	if err != nil {
		t.Fatalf("failed to reconnect Agent WS to Node C: %v", err)
	}
	defer agentWSConnC.Close()

	if err := performAgentHandshake(agentWSConnC); err != nil {
		t.Fatalf("failed agent challenge handshake on Node C: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	newOwner, err := agentRepo.GetOwner(ctx, orgID, deviceID)
	if err != nil || newOwner == nil || newOwner.NodeID != "node-c" || newOwner.Generation != 2 {
		t.Fatalf("expected failover to Node C gen 2, got: %v", newOwner)
	}

	// 6. Route 2nd command from Node B to Node C -> Agent on Node C succeeds!
	snap2 := agentws.ConnectionSnapshot{
		AgentID:      newOwner.AgentID,
		ConnectionID: newOwner.ConnectionID,
		Generation:   newOwner.Generation,
	}

	cmdID2 := "cmd_e2e_1002"
	cmdEnvelope2 := map[string]interface{}{
		"command_id":      cmdID2,
		"type":            "input.swipe",
		"attempt_number":  1,
		"status":          "prepared",
		"expires_at_unix": time.Now().Add(5 * time.Minute).Unix(),
	}
	cmdBytes2, _ := json.Marshal(cmdEnvelope2)

	err = routerB.DispatchCommandRoute(ctx, orgID, deviceID, snap2, newOwner.NodeID, cmdBytes2)
	if err != nil {
		t.Fatalf("expected routerB.DispatchCommandRoute to succeed post-failover, got: %v", err)
	}

	_ = agentWSConnC.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, rMsgBytesC2, err := agentWSConnC.ReadMessage()
	if err != nil {
		t.Fatalf("expected Agent WS on Node C to receive 2nd command after failover, got: %v", err)
	}

	var recCmd2 map[string]interface{}
	_ = json.Unmarshal(rMsgBytesC2, &recCmd2)
	if recCmd2["command_id"] != cmdID2 {
		t.Fatalf("expected 2nd command_id %s on Node C Agent socket, got: %v", cmdID2, recCmd2)
	}
}
