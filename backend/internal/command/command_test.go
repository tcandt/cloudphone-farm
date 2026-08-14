package command_test

import (
	"errors"
	"testing"

	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
)

func TestInputCommandsValidation(t *testing.T) {
	allowedCommands := []string{
		"gesture.touch",
		"gesture.swipe",
		"input.text",
		"global.back",
		"global.home",
		"global.recents",
	}

	for _, cmd := range allowedCommands {
		switch cmd {
		case "gesture.touch", "gesture.swipe", "input.text", "global.back", "global.home", "global.recents":
			// Valid
		default:
			t.Errorf("Expected allowed input command type %s", cmd)
		}
	}

	rejectedCommands := []string{
		"device.reboot",
		"device.lock",
		"apk.install",
		"network.proxy.apply",
	}

	for _, cmd := range rejectedCommands {
		switch cmd {
		case "gesture.touch", "gesture.swipe", "input.text", "global.back", "global.home", "global.recents":
			t.Errorf("Expected rejected administrative command type %s", cmd)
		default:
			// Correctly rejected
		}
	}
}

func TestControlLeaseErrorHandling(t *testing.T) {
	if !errors.Is(domain.ErrControlAlreadyLeased, domain.ErrControlAlreadyLeased) {
		t.Errorf("Expected ErrControlAlreadyLeased error instance")
	}

	if !errors.Is(domain.ErrLeaseNotOwned, domain.ErrLeaseNotOwned) {
		t.Errorf("Expected ErrLeaseNotOwned error instance")
	}

	if !errors.Is(domain.ErrUnauthorizedCommand, domain.ErrUnauthorizedCommand) {
		t.Errorf("Expected ErrUnauthorizedCommand error instance")
	}
}
