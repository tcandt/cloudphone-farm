package agent

import (
	"context"
	"encoding/json"
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

func TestHeartbeatRequestDTO_NullableTelemetryAndKeyProtection(t *testing.T) {
	t.Run("parses missing telemetry as nil pointers (unknown != zero)", func(t *testing.T) {
		jsonStr := `{"connection_id":"conn_1","generation":2,"sequence":5}`
		var req HeartbeatRequestDTO
		if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if req.CPUUsage != nil {
			t.Fatalf("expected CPUUsage to be nil, got: %v", *req.CPUUsage)
		}
		if req.TemperatureC != nil {
			t.Fatalf("expected TemperatureC to be nil, got: %v", *req.TemperatureC)
		}
		if req.Battery != nil {
			t.Fatalf("expected Battery to be nil, got: %v", *req.Battery)
		}
		if req.Network != nil {
			t.Fatalf("expected Network to be nil, got: %v", *req.Network)
		}
	})

	t.Run("parses actual key protection metadata", func(t *testing.T) {
		jsonStr := `{
			"connection_id":"conn_1",
			"generation":2,
			"sequence":5,
			"key_protection": {
				"algorithm": "AES-256-GCM",
				"provider": "AndroidKeyStore",
				"security_level": "STRONGBOX"
			}
		}`
		var req HeartbeatRequestDTO
		if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
			t.Fatalf("failed to unmarshal JSON: %v", err)
		}

		if req.KeyProtection == nil {
			t.Fatalf("expected KeyProtection to be non-nil")
		}
		if req.KeyProtection.SecurityLevel != "STRONGBOX" {
			t.Fatalf("expected SecurityLevel STRONGBOX, got: %s", req.KeyProtection.SecurityLevel)
		}
	})
}
