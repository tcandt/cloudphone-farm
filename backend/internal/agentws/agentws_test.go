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

func TestHubRegistrationAndBackpressure(t *testing.T) {
	hub := agentws.NewHub()
	agentID := "agt_test_01"

	// Verify unregistered dispatch returns error
	err := hub.DispatchToAgent(agentID, []byte("test"))
	if err == nil || !errors.Is(err, agentws.ErrAgentNotConnected) {
		t.Errorf("Expected ErrAgentNotConnected for unregistered agent, got %v", err)
	}

	// Test generation counter
	gen1 := hub.NextGeneration(agentID)
	gen2 := hub.NextGeneration(agentID)
	if gen2 != gen1+1 {
		t.Errorf("Expected generation counter to increment, got gen1=%d gen2=%d", gen1, gen2)
	}
}
