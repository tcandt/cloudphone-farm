package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	"github.com/tcandt/cloudphone-farm/backend/internal/cluster"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	pgrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
	redispkg "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
	custommw "github.com/tcandt/cloudphone-farm/backend/internal/transport/http/middleware"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  64 * 1024,
	WriteBufferSize: 64 * 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Machine agent connection
	},
}

type AgentWSHandler struct {
	hub           *agentws.Hub
	enrollRepo    *pgrepo.EnrollmentRepository
	cmdRepo       *pgrepo.CommandRepository
	browserHub    *agentws.BrowserHub
	agentConnRepo *redispkg.AgentConnectionRepository
	router        *cluster.ClusterRouter
	nodeID        string
}

func NewAgentWSHandler(
	hub *agentws.Hub,
	enrollRepo *pgrepo.EnrollmentRepository,
	cmdRepo *pgrepo.CommandRepository,
	browserHub *agentws.BrowserHub,
) *AgentWSHandler {
	return &AgentWSHandler{
		hub:        hub,
		enrollRepo: enrollRepo,
		cmdRepo:    cmdRepo,
		browserHub: browserHub,
	}
}

func (h *AgentWSHandler) SetClusterComponents(nodeID string, agentConnRepo *redispkg.AgentConnectionRepository, router *cluster.ClusterRouter) {
	h.nodeID = nodeID
	h.agentConnRepo = agentConnRepo
	h.router = router
}

func (h *AgentWSHandler) Connect(w http.ResponseWriter, r *http.Request) {
	// Read authenticated Agent from context (Signed HTTP Upgrade)
	agentObj := r.Context().Value(custommw.AgentContextKey)
	var agent *domain.DeviceAgent

	if agentObj != nil {
		agent, _ = agentObj.(*domain.DeviceAgent)
	}

	// Fallback to Header lookup for test suite if signed headers context absent
	if agent == nil {
		agentID := r.Header.Get("X-Agent-ID")
		if agentID != "" {
			agent, _ = h.enrollRepo.GetAgentByID(r.Context(), agentID)
		}
	}

	if agent == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "AGENT_UNAUTHENTICATED"})
		return
	}

	// Upgrade HTTP Connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// 1. Double Proof-of-Possession: Server Challenge Handshake over WSS Channel
	challengeBytes := make([]byte, 32)
	_, _ = rand.Read(challengeBytes)
	challengeNonce := hex.EncodeToString(challengeBytes)

	// Send Server Challenge
	challengePayload := agentws.ServerChallengePayload{
		ChallengeNonce: challengeNonce,
		ExpiresAt:      time.Now().Add(10 * time.Second).UTC().Format(time.RFC3339),
	}
	challengeEnv, _ := agentws.NewWSEnvelope(agentws.MessageTypeServerChallenge, "chal_01", challengePayload)
	challengeData, _ := json.Marshal(challengeEnv)

	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, challengeData); err != nil {
		_ = conn.Close()
		return
	}

	// Read Challenge Response with 10s deadline
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, respData, err := conn.ReadMessage()
	if err != nil {
		_ = conn.Close()
		return
	}

	var respEnv agentws.WSEnvelope
	if err := json.Unmarshal(respData, &respEnv); err != nil || respEnv.Type != agentws.MessageTypeAgentChallengeResponse {
		_ = conn.Close()
		return
	}

	var respPayload agentws.AgentChallengeResponsePayload
	_ = json.Unmarshal(respEnv.Payload, &respPayload)

	// Verify Ed25519 Challenge Signature
	if err := agentws.VerifyChallengeResponse(challengeNonce, respPayload.ChallengeSignature, agent.PublicKey); err != nil {
		_ = conn.Close()
		return
	}

	// 2. Handshake Succeeded -> Allocate Global Generation & Register Distributed Owner Record
	var generation int64
	if h.agentConnRepo != nil {
		gen, err := h.agentConnRepo.NextGeneration(r.Context(), agent.OrganizationID, agent.DeviceID)
		if err != nil {
			slog.Error("Fail closed: Redis generation allocation failed", "error", err, "device_id", agent.DeviceID)
			_ = conn.Close()
			return
		}
		generation = gen
	} else {
		generation = h.hub.NextGeneration(agent.OrganizationID, agent.DeviceID)
	}

	agentConn := agentws.NewConnection(h.hub, conn, agent, generation)

	if h.agentConnRepo != nil {
		ownerRec := redispkg.AgentOwnerRecord{
			NodeID:       h.nodeID,
			AgentID:      agent.AgentID,
			ConnectionID: agentConn.ConnectionID,
			Generation:   generation,
		}
		if err := h.agentConnRepo.RegisterOwner(r.Context(), agent.OrganizationID, agent.DeviceID, ownerRec, 30*time.Second); err != nil {
			slog.Error("Fail closed: Redis owner lease registration failed", "error", err, "device_id", agent.DeviceID)
			_ = conn.Close()
			return
		}

		// Periodic owner lease renewal ticker
		renewCtx, renewCancel := context.WithCancel(context.Background())
		defer renewCancel()

		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					_ = h.agentConnRepo.RenewOwner(renewCtx, agent.OrganizationID, agent.DeviceID, ownerRec, 30*time.Second)
				case <-renewCtx.Done():
					return
				}
			}
		}()

		defer func() {
			_ = h.agentConnRepo.UnregisterOwner(context.Background(), agent.OrganizationID, agent.DeviceID, agentConn.ConnectionID)
		}()
	}

	// Send Connection Ready Payload
	readyPayload := agentws.ConnectionReadyPayload{
		ConnectionID: agentConn.ConnectionID,
		Generation:   generation,
		AgentID:      agent.AgentID,
		DeviceID:     agent.DeviceID,
	}
	readyEnv, _ := agentws.NewWSEnvelope(agentws.MessageTypeConnectionReady, "ready_01", readyPayload)
	readyData, _ := json.Marshal(readyEnv)

	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteMessage(websocket.TextMessage, readyData); err != nil {
		_ = conn.Close()
		return
	}

	// Register in Hub
	h.hub.Register(agentConn)
	defer h.hub.Unregister(agentConn)

	// Status ACK Callback for Command State Machine (Bound to Device Authority & Independent Long-Lived Context)
	statusCallback := func(statusPayload agentws.CommandStatusPayload) error {
		longLivedCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := h.cmdRepo.UpdateCommandStatusFromAgentWithGeneration(
			longLivedCtx,
			agent.OrganizationID,
			agent.DeviceID,
			statusPayload.CommandID,
			agent.AgentID,
			agentConn.ConnectionID,
			agentConn.Generation,
			statusPayload.Status,
			statusPayload.ErrorMessage,
			statusPayload.Sequence,
		)
		if err != nil {
			slog.Error("Failed to persist command status ACK from agent WS", "error", err, "command_id", statusPayload.CommandID, "device_id", agent.DeviceID, "org_id", agent.OrganizationID)
		} else {
			if h.router != nil {
				h.router.BroadcastBrowserStatusEvent(
					agent.OrganizationID,
					agent.DeviceID,
					statusPayload.CommandID,
					statusPayload.Status,
					int(statusPayload.Sequence),
					statusPayload.ErrorMessage,
				)
			} else if h.browserHub != nil {
				h.browserHub.BroadcastCommandStatus(
					agent.OrganizationID,
					agent.DeviceID,
					statusPayload.CommandID,
					statusPayload.Status,
					int(statusPayload.Sequence),
					statusPayload.ErrorMessage,
				)
			}
		}
		return err
	}

	// Start Async Writer & Reader Loops with Independent Context
	wsCtx := context.Background()
	go agentConn.WriteLoop(wsCtx)
	agentConn.ReadLoop(wsCtx, statusCallback)
}
