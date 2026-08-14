package command

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	pgrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
)

type OutboxDispatcher struct {
	outboxRepo  *pgrepo.OutboxRepository
	cmdRepo     *pgrepo.CommandRepository
	wsHub       *agentws.Hub
	workerID    string
	stopChan    chan struct{}
	wg          sync.WaitGroup
	maxAttempts int
}

func NewOutboxDispatcher(outboxRepo *pgrepo.OutboxRepository, cmdRepo *pgrepo.CommandRepository, wsHub *agentws.Hub) *OutboxDispatcher {
	workerID := fmt.Sprintf("worker_%s", uuid.New().String()[:8])
	return &OutboxDispatcher{
		outboxRepo:  outboxRepo,
		cmdRepo:     cmdRepo,
		wsHub:       wsHub,
		workerID:    workerID,
		stopChan:    make(chan struct{}),
		maxAttempts: 5,
	}
}

func (d *OutboxDispatcher) Start(ctx context.Context) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		slog.Info("Outbox Dispatcher worker started", "worker_id", d.workerID)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				d.processBatch(ctx)
			case <-d.stopChan:
				slog.Info("Outbox Dispatcher worker stopping", "worker_id", d.workerID)
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (d *OutboxDispatcher) Stop() {
	close(d.stopChan)
	d.wg.Wait()
}

func (d *OutboxDispatcher) processBatch(ctx context.Context) {
	messages, err := d.outboxRepo.ClaimPendingOutboxMessages(ctx, d.workerID, 50)
	if err != nil || len(messages) == 0 {
		return
	}

	for _, msg := range messages {
		d.dispatchSingleMessage(ctx, msg)
	}
}

func (d *OutboxDispatcher) dispatchSingleMessage(ctx context.Context, msg pgrepo.OutboxMessage) {
	agentID := fmt.Sprintf("agt_%s", msg.DeviceID)

	// 1. Fetch Command Record to check TTL / expiration
	cmdRec, err := d.cmdRepo.GetCommandByID(ctx, msg.CommandID)
	if err == nil && cmdRec != nil && time.Now().After(cmdRec.ExpiresAt) {
		slog.Warn("Command expired before outbox dispatch", "command_id", msg.CommandID, "expires_at", cmdRec.ExpiresAt)
		_ = d.outboxRepo.UpdateOutboxFailed(ctx, msg.OutboxID, "command TTL expired before dispatch")
		_ = d.cmdRepo.UpdateCommandStatus(ctx, msg.CommandID, "expired", "command TTL expired before dispatch")
		return
	}

	// 2. Construct WS Command Dispatch Envelope
	dispatchPayload := agentws.CommandDispatchPayload{
		CommandID:   msg.CommandID,
		DeviceID:    msg.DeviceID,
		CommandType: "gesture.touch",
		IssuedAt:    msg.CreatedAt.UTC().Format(time.RFC3339),
	}
	if cmdRec != nil {
		dispatchPayload.CommandType = cmdRec.CommandType
		dispatchPayload.ExpiresAt = cmdRec.ExpiresAt.UTC().Format(time.RFC3339)
	}

	env, err := agentws.NewWSEnvelope(agentws.MessageTypeCommandDispatch, fmt.Sprintf("msg_%d", time.Now().UnixNano()), dispatchPayload)
	if err != nil {
		slog.Error("Failed to construct WS envelope", "error", err)
		return
	}

	envData, _ := json.Marshal(env)

	// 3. Attempt Dispatch to Agent WebSocket Hub
	err = d.wsHub.DispatchToAgent(agentID, envData)
	if err != nil {
		// Retries with exponential backoff: 1s, 2s, 4s, 8s
		if msg.AttemptCount+1 >= d.maxAttempts {
			slog.Error("Outbox message permanently failed max retry limit", "outbox_id", msg.OutboxID, "agent_id", agentID, "attempts", msg.AttemptCount+1)
			_ = d.outboxRepo.UpdateOutboxFailed(ctx, msg.OutboxID, err.Error())
			_ = d.cmdRepo.UpdateCommandStatus(ctx, msg.CommandID, "failed", fmt.Sprintf("Agent offline after %d attempts", d.maxAttempts))
		} else {
			backoffSec := 1 << msg.AttemptCount // 1, 2, 4, 8...
			nextAttempt := time.Now().Add(time.Duration(backoffSec) * time.Second)
			slog.Warn("Outbox message dispatch failed, scheduling retry", "outbox_id", msg.OutboxID, "agent_id", agentID, "next_attempt", nextAttempt, "error", err)
			_ = d.outboxRepo.UpdateOutboxRetry(ctx, msg.OutboxID, err.Error(), nextAttempt)
		}
		return
	}

	// 4. Dispatch Succeeded -> Mark Outbox as Dispatched (CRITICAL: DOES NOT MARK COMMAND SUCCEEDED!)
	slog.Info("Successfully dispatched outbox command to Agent WS Hub", "command_id", msg.CommandID, "agent_id", agentID)
	_ = d.outboxRepo.UpdateOutboxDispatched(ctx, msg.OutboxID)
	_ = d.cmdRepo.UpdateCommandStatus(ctx, msg.CommandID, "ack", "")
}
