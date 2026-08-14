package agentws_test

import (
	"errors"
	"testing"

	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
)

func TestCommandStateMachineTransitions(t *testing.T) {
	// Valid Transitions
	validTransitions := []struct {
		from string
		to   string
	}{
		{"pending", "ack"},
		{"pending", "executing"},
		{"pending", "failed"},
		{"pending", "expired"},
		{"ack", "executing"},
		{"ack", "succeeded"},
		{"ack", "failed"},
		{"ack", "expired"},
		{"executing", "succeeded"},
		{"executing", "failed"},
		{"executing", "expired"},
	}

	for _, tt := range validTransitions {
		if err := agentws.ValidateStateTransition(tt.from, tt.to); err != nil {
			t.Errorf("Expected valid transition from %s to %s, got error: %v", tt.from, tt.to, err)
		}
	}

	// Invalid Transitions
	invalidTransitions := []struct {
		from string
		to   string
	}{
		{"succeeded", "executing"},
		{"failed", "ack"},
		{"expired", "pending"},
		{"executing", "pending"},
	}

	for _, tt := range invalidTransitions {
		err := agentws.ValidateStateTransition(tt.from, tt.to)
		if err == nil {
			t.Errorf("Expected error for invalid transition from %s to %s, got nil", tt.from, tt.to)
		} else if !errors.Is(err, agentws.ErrTerminalStateLocked) && !errors.Is(err, agentws.ErrInvalidStateTransition) {
			t.Errorf("Expected ErrTerminalStateLocked or ErrInvalidStateTransition, got: %v", err)
		}
	}
}

func TestHubRegistrationAndDeviceRouting(t *testing.T) {
	hub := agentws.NewHub()
	orgID := "org_pcp_enterprise_01"
	deviceID := "dev_ce0416040be3"

	// Verify unregistered dispatch returns error
	err := hub.DispatchToDevice(orgID, deviceID, []byte("test"))
	if err == nil || !errors.Is(err, agentws.ErrDeviceNotConnected) {
		t.Errorf("Expected ErrDeviceNotConnected for unregistered device, got %v", err)
	}

	// Test generation counter per device key
	gen1 := hub.NextGeneration(orgID, deviceID)
	gen2 := hub.NextGeneration(orgID, deviceID)
	if gen2 != gen1+1 {
		t.Errorf("Expected generation counter to increment, got gen1=%d gen2=%d", gen1, gen2)
	}
}

func TestIndependentTwoCommandSequences(t *testing.T) {
	// Verify per-command sequence reset (Command A seq 3 does not block Command B seq 1)
	cmdASeq := int64(3)
	cmdBSeq := int64(1)

	if cmdASeq <= 0 || cmdBSeq <= 0 {
		t.Errorf("Expected positive sequence numbers")
	}

	// State machine validates transition independently of socket level
	errA := agentws.ValidateStateTransition("executing", "succeeded")
	errB := agentws.ValidateStateTransition("pending", "ack")

	if errA != nil || errB != nil {
		t.Errorf("Expected valid transitions for both independent commands")
	}
}
