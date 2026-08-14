package agent

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type ExecutionResult struct {
	CommandID string `json:"command_id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

type CommandExecutor struct {
	journal *CommandJournal
	fencing *FencingManager
}

func NewCommandExecutor(journal *CommandJournal, fencing *FencingManager) *CommandExecutor {
	return &CommandExecutor{
		journal: journal,
		fencing: fencing,
	}
}

func (e *CommandExecutor) Execute(ctx context.Context, commandID string, fencingToken int64, cmdType string, payload map[string]interface{}) (*ExecutionResult, error) {
	// 1. Deduplication Check against Command Journal
	if existing, found := e.journal.Get(commandID); found {
		slog.Info("Android Agent deduplicated duplicate command dispatch", "command_id", commandID, "status", existing.Status)
		return &ExecutionResult{
			CommandID: commandID,
			Status:    existing.Status,
			Error:     existing.ErrorMessage,
		}, nil
	}

	// 2. Fencing Token Validation
	if !e.fencing.ValidateAndUpdate(fencingToken) {
		errStr := fmt.Sprintf("stale fencing token %d rejected (highest known: %d)", fencingToken, e.fencing.GetHighest())
		slog.Warn("Android Agent rejected stale fencing token", "command_id", commandID, "err", errStr)
		e.journal.Record(commandID, fencingToken, "failed", errStr)
		return &ExecutionResult{
			CommandID: commandID,
			Status:    "failed",
			Error:     errStr,
		}, fmt.Errorf(errStr)
	}

	// 3. Physical Input Execution Dispatch
	slog.Info("Android Agent executing input command", "command_id", commandID, "type", cmdType)
	var execErr error

	switch cmdType {
	case "gesture.touch":
		execErr = e.executeTouch(payload)

	case "gesture.swipe":
		execErr = e.executeSwipe(payload)

	case "input.text":
		execErr = e.executeText(payload)

	case "global.back", "global.home", "global.recents":
		execErr = e.executeNavigation(cmdType)

	default:
		execErr = fmt.Errorf("unsupported input command type: %s", cmdType)
	}

	status := "succeeded"
	errMessage := ""
	if execErr != nil {
		status = "failed"
		errMessage = execErr.Error()
	}

	e.journal.Record(commandID, fencingToken, status, errMessage)

	return &ExecutionResult{
		CommandID: commandID,
		Status:    status,
		Error:     errMessage,
	}, execErr
}

func (e *CommandExecutor) executeTouch(payload map[string]interface{}) error {
	x, _ := parseCoordinate(payload["x"])
	y, _ := parseCoordinate(payload["y"])
	slog.Info("Executing physical touch gesture on Android device", "x", x, "y", y)
	time.Sleep(10 * time.Millisecond) // Simulated accessibility touch dispatch
	return nil
}

func (e *CommandExecutor) executeSwipe(payload map[string]interface{}) error {
	startX, _ := parseCoordinate(payload["startX"])
	startY, _ := parseCoordinate(payload["startY"])
	endX, _ := parseCoordinate(payload["endX"])
	endY, _ := parseCoordinate(payload["endY"])
	durationMs, _ := parseCoordinate(payload["durationMs"])
	slog.Info("Executing physical swipe gesture on Android device", "startX", startX, "startY", startY, "endX", endX, "endY", endY, "durationMs", durationMs)
	time.Sleep(20 * time.Millisecond)
	return nil
}

func (e *CommandExecutor) executeText(payload map[string]interface{}) error {
	txt, _ := payload["text"].(string)
	slog.Info("Executing physical text input on Android device", "length", len(txt))
	time.Sleep(5 * time.Millisecond)
	return nil
}

func (e *CommandExecutor) executeNavigation(action string) error {
	slog.Info("Executing physical navigation keycode on Android device", "action", action)
	time.Sleep(5 * time.Millisecond)
	return nil
}

func parseCoordinate(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		return 0, false
	}
}
