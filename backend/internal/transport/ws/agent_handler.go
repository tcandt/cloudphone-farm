package ws

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	pgrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  64 * 1024,
	WriteBufferSize: 64 * 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Machine agent connection
	},
}

type AgentWSHandler struct {
	hub        *agentws.Hub
	enrollRepo *pgrepo.EnrollmentRepository
	cmdRepo    *pgrepo.CommandRepository
}

func NewAgentWSHandler(hub *agentws.Hub, enrollRepo *pgrepo.EnrollmentRepository, cmdRepo *pgrepo.CommandRepository) *AgentWSHandler {
	return &AgentWSHandler{
		hub:        hub,
		enrollRepo: enrollRepo,
		cmdRepo:    cmdRepo,
	}
}

func (h *AgentWSHandler) Connect(w http.ResponseWriter, r *http.Request) {
	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	// 1. Read Challenge Response Handshake
	agentID := r.Header.Get("X-Agent-ID")
	if agentID == "" {
		_ = conn.Close()
		return
	}

	agent, err := h.enrollRepo.GetAgentByID(r.Context(), agentID)
	if err != nil || agent == nil {
		_ = conn.Close()
		return
	}

	// Generate Server Challenge Nonce
	challengeBytes := make([]byte, 32)
	_, _ = rand.Read(challengeBytes)
	challengeNonce := hex.EncodeToString(challengeBytes)

	// Send Server Challenge
	challengePayload := agentws.ServerChallengePayload{
		ChallengeNonce: challengeNonce,
	}
	challengeEnv, _ := agentws.NewWSEnvelope(agentws.MessageTypeServerChallenge, "chal_01", challengePayload)
	challengeData, _ := json.Marshal(challengeEnv)
	_ = conn.WriteMessage(websocket.TextMessage, challengeData)

	// Read Challenge Response
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

	// 2. Handshake Succeeded -> Initialize Agent Connection
	generation := h.hub.NextGeneration(agent.AgentID)
	agentConn := agentws.NewConnection(h.hub, conn, agent, generation)

	// Send Connection Ready
	readyPayload := agentws.ConnectionReadyPayload{
		ConnectionID: agentConn.ConnectionID,
		Generation:   generation,
		AgentID:      agent.AgentID,
		DeviceID:     agent.DeviceID,
	}
	readyEnv, _ := agentws.NewWSEnvelope(agentws.MessageTypeConnectionReady, "ready_01", readyPayload)
	readyData, _ := json.Marshal(readyEnv)
	_ = conn.WriteMessage(websocket.TextMessage, readyData)

	// Register in Hub
	h.hub.Register(agentConn)

	// Status ACK Callback for Command State Machine
	statusCallback := func(statusPayload agentws.CommandStatusPayload) error {
		return h.cmdRepo.UpdateCommandStatus(r.Context(), statusPayload.CommandID, statusPayload.Status, statusPayload.ErrorMessage)
	}

	// Start Async Writer & Reader Loops
	go agentConn.WriteLoop(r.Context())
	agentConn.ReadLoop(r.Context(), statusCallback)
}
