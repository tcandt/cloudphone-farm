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
	browserHub  *agentws.BrowserHub
	workerID    string
	stopChan    chan struct{}
	wg          sync.WaitGroup
	maxAttempts int
}

func NewOutboxDispatcher(outboxRepo *pgrepo.OutboxRepository, cmdRepo *pgrepo.CommandRepository, wsHub *agentws.Hub, browserHub *agentws.BrowserHub) *OutboxDispatcher {
	workerID := fmt.Sprintf("worker_%s", uuid.New().String()[:8])
	return &OutboxDispatcher{
		outboxRepo:  outboxRepo,
		cmdRepo:     cmdRepo,
		wsHub:       wsHub,
		browserHub:  browserHub,
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
	cmdRec, err := d.cmdRepo.GetCommandByID(ctx, msg.CommandID)
	if err != nil || cmdRec == nil {
		slog.Error("Failed to fetch command record from DB, skipping dispatch for retry", "err", err, "command_id", msg.CommandID)
		_ = d.outboxRepo.UpdateOutboxRetry(ctx, msg.OutboxID, "DB query error fetching command", time.Now().Add(1*time.Second))
		return
	}

	if time.Now().After(cmdRec.ExpiresAt) {
		slog.Warn("Command expired before outbox dispatch", "command_id", msg.CommandID, "expires_at", cmdRec.ExpiresAt)
		_ = d.outboxRepo.UpdateOutboxFailed(ctx, msg.OutboxID, "command TTL expired before dispatch")
		_ = d.cmdRepo.UpdateCommandStatus(ctx, msg.CommandID, "expired", "command TTL expired before dispatch", 0)
		return
	}

	var rawPayload map[string]interface{}
	if len(msg.PayloadJSON) > 0 {
		_ = json.Unmarshal(msg.PayloadJSON, &rawPayload)
	}
	if rawPayload == nil && len(cmdRec.PayloadJSON) > 0 {
		_ = json.Unmarshal(cmdRec.PayloadJSON, &rawPayload)
	}

	var fencingToken int64
	var controlLeaseID string

	if rawPayload != nil {
		if ft, ok := rawPayload["fencing_token"].(float64); ok {
			fencingToken = int64(ft)
		} else if ft, ok := rawPayload["fencing_token"].(int64); ok {
			fencingToken = ft
		}
		if cl, ok := rawPayload["control_lease_id"].(string); ok {
			controlLeaseID = cl
		}
	}

	dispatchPayload := agentws.CommandDispatchPayload{
		CommandID:    msg.CommandID,
		DeviceID:     msg.DeviceID,
		CommandType:  cmdRec.CommandType,
		Payload:      rawPayload,
		ControlLease: controlLeaseID,
		FencingToken: fencingToken,
		IssuedAt:     msg.CreatedAt.UTC().Format(time.RFC3339),
		ExpiresAt:    cmdRec.ExpiresAt.UTC().Format(time.RFC3339),
	}

	env, err := agentws.NewWSEnvelope(agentws.MessageTypeCommandDispatch, fmt.Sprintf("msg_%d", time.Now().UnixNano()), dispatchPayload)
	if err != nil {
		slog.Error("Failed to construct WS envelope", "error", err)
		return
	}

	envData, _ := json.Marshal(env)

	// Snapshot active connection generation authority
	snap, ok := d.wsHub.GetConnectionSnapshot(msg.OrganizationID, msg.DeviceID)
	if !ok {
		slog.Warn("Device agent not connected to hub for outbox dispatch", "device_id", msg.DeviceID, "command_id", msg.CommandID)
		if msg.AttemptCount+1 >= d.maxAttempts {
			_ = d.outboxRepo.UpdateOutboxFailed(ctx, msg.OutboxID, "Device offline after max attempts")
			_ = d.cmdRepo.UpdateCommandStatus(ctx, msg.CommandID, "failed", fmt.Sprintf("Device offline after %d attempts", d.maxAttempts), 0)
		} else {
			backoffSec := 1 << msg.AttemptCount
			_ = d.outboxRepo.UpdateOutboxRetry(ctx, msg.OutboxID, "Device agent not connected", time.Now().Add(time.Duration(backoffSec)*time.Second))
		}
		return
	}

	// Dispatch to exact snapshot
	err = d.wsHub.DispatchToConnectionSnapshot(msg.OrganizationID, msg.DeviceID, snap, envData)
	if err != nil {
		if msg.AttemptCount+1 >= d.maxAttempts {
			slog.Error("Outbox message permanently failed max retry limit", "outbox_id", msg.OutboxID, "device_id", msg.DeviceID, "attempts", msg.AttemptCount+1)
			_ = d.outboxRepo.UpdateOutboxFailed(ctx, msg.OutboxID, err.Error())
			_ = d.cmdRepo.UpdateCommandStatus(ctx, msg.CommandID, "failed", fmt.Sprintf("Device offline after %d attempts", d.maxAttempts), 0)
		} else {
			backoffSec := 1 << msg.AttemptCount
			nextAttempt := time.Now().Add(time.Duration(backoffSec) * time.Second)
			slog.Warn("Outbox message dispatch failed, scheduling retry", "outbox_id", msg.OutboxID, "device_id", msg.DeviceID, "next_attempt", nextAttempt, "error", err)
			_ = d.outboxRepo.UpdateOutboxRetry(ctx, msg.OutboxID, err.Error(), nextAttempt)
		}
		return
	}

	attemptNo := msg.AttemptCount + 1
	_ = d.cmdRepo.RecordDeliveryAttempt(ctx, msg.OrganizationID, msg.CommandID, msg.DeviceID, attemptNo, snap.AgentID, snap.ConnectionID, snap.Generation)
	_ = d.outboxRepo.UpdateOutboxDispatched(ctx, msg.OutboxID)

	if d.browserHub != nil {
		d.browserHub.BroadcastCommandDelivery(msg.OrganizationID, msg.DeviceID, msg.CommandID, "dispatched", attemptNo)
	}
}
