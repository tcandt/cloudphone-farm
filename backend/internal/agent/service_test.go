package agent

import (
	"context"
	"testing"

	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	redisrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
)

func TestProcessHeartbeat_FencingValidation(t *testing.T) {
	svc := NewAgentService(nil, nil, nil)

	agent := &domain.DeviceAgent{
		OrganizationID: "org_test",
		DeviceID:       "dev_test",
		AgentID:        "agent_test",
	}

	t.Run("nil agentConnRepo returns error", func(t *testing.T) {
		err := svc.ProcessHeartbeat(context.Background(), agent, HeartbeatRequestDTO{
			ConnectionID: "conn_123",
			Generation:   1,
		})
		if err == nil || err.Error() != "heartbeat authority failure: agent connection repository is unavailable" {
			t.Fatalf("expected repo unavailable error, got: %v", err)
		}
	})

	t.Run("empty connection_id fails closed", func(t *testing.T) {
		connRepo := redisrepo.NewAgentConnectionRepository(nil)
		svc.SetAgentConnectionRepository(connRepo)

		err := svc.ProcessHeartbeat(context.Background(), agent, HeartbeatRequestDTO{
			ConnectionID: "",
			Generation:   1,
		})
		if err == nil || err.Error() != "heartbeat owner mismatch: connection_id and generation must be provided" {
			t.Fatalf("expected connection_id missing error, got: %v", err)
		}
	})

	t.Run("zero generation fails closed", func(t *testing.T) {
		connRepo := redisrepo.NewAgentConnectionRepository(nil)
		svc.SetAgentConnectionRepository(connRepo)

		err := svc.ProcessHeartbeat(context.Background(), agent, HeartbeatRequestDTO{
			ConnectionID: "conn_123",
			Generation:   0,
		})
		if err == nil || err.Error() != "heartbeat owner mismatch: connection_id and generation must be provided" {
			t.Fatalf("expected generation missing error, got: %v", err)
		}
	})
}
