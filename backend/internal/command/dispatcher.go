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
	"github.com/tcandt/cloudphone-farm/backend/internal/cluster"
	pgrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
	redispkg "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
	"github.com/tcandt/cloudphone-farm/backend/internal/telemetry"
)

type OutboxDispatcher struct {
	outboxRepo    *pgrepo.OutboxRepository
	cmdRepo       *pgrepo.CommandRepository
	wsHub         *agentws.Hub
	browserHub    *agentws.BrowserHub
	agentConnRepo *redispkg.AgentConnectionRepository
	router        *cluster.ClusterRouter
	workerID      string
	stopChan      chan struct{}
	wg            sync.WaitGroup
	maxAttempts   int
	mu            sync.RWMutex
	isRunning     bool
	lastLoopAt    time.Time
	lastError     string
}

func NewOutboxDispatcher(
	outboxRepo *pgrepo.OutboxRepository,
	cmdRepo *pgrepo.CommandRepository,
	wsHub *agentws.Hub,
	browserHub *agentws.BrowserHub,
) *OutboxDispatcher {
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

func (d *OutboxDispatcher) SetClusterComponents(agentConnRepo *redispkg.AgentConnectionRepository, router *cluster.ClusterRouter) {
	d.agentConnRepo = agentConnRepo
	d.router = router
}

func (d *OutboxDispatcher) Start(ctx context.Context) {
	d.mu.Lock()
	d.isRunning = true
	d.lastLoopAt = time.Now()
	d.mu.Unlock()

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		defer func() {
			d.mu.Lock()
			d.isRunning = false
			d.mu.Unlock()
		}()

		slog.Info("Outbox Dispatcher worker started", "worker_id", d.workerID)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				d.mu.Lock()
				d.lastLoopAt = time.Now()
				d.mu.Unlock()
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
	d.mu.Lock()
	d.isRunning = false
	d.mu.Unlock()
}

func (d *OutboxDispatcher) GetWorkerStatus() (bool, time.Time, string) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.isRunning, d.lastLoopAt, d.lastError
}

func (d *OutboxDispatcher) processBatch(ctx context.Context) {
	messages, err := d.outboxRepo.ClaimPendingOutboxMessages(ctx, d.workerID, 50)
	if err != nil {
		d.mu.Lock()
		d.lastError = err.Error()
		d.mu.Unlock()
		return
	}
	if len(messages) == 0 {
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

	attemptNo := msg.AttemptCount + 1

	var targetAgentID string
	var targetConnID string
	var targetGen int64
	var targetNodeID string

	// 1. Resolve agent owner authority from Redis or local snapshot
	if d.agentConnRepo != nil {
		owner, err := d.agentConnRepo.GetOwner(ctx, msg.OrganizationID, msg.DeviceID)
		if err != nil || owner == nil {
			slog.Warn("Device agent owner not found in distributed directory", "device_id", msg.DeviceID, "command_id", msg.CommandID, "err", err)
			d.handleDispatchOffline(ctx, msg, attemptNo)
			return
		}
		targetAgentID = owner.AgentID
		targetConnID = owner.ConnectionID
		targetGen = owner.Generation
		targetNodeID = owner.NodeID
	} else {
		snap, ok := d.wsHub.GetConnectionSnapshot(msg.OrganizationID, msg.DeviceID)
		if !ok {
			slog.Warn("Device agent not connected to local hub", "device_id", msg.DeviceID, "command_id", msg.CommandID)
			d.handleDispatchOffline(ctx, msg, attemptNo)
			return
		}
		targetAgentID = snap.AgentID
		targetConnID = snap.ConnectionID
		targetGen = snap.Generation
		targetNodeID = ""
	}

	snap := agentws.ConnectionSnapshot{
		AgentID:      targetAgentID,
		ConnectionID: targetConnID,
		Generation:   targetGen,
	}

	// 2. PERSIST delivery attempt with status 'prepared' BEFORE socket dispatch
	err = d.cmdRepo.RecordDeliveryAttempt(ctx, msg.OrganizationID, msg.CommandID, msg.DeviceID, attemptNo, targetAgentID, targetConnID, targetGen, "prepared")
	if err != nil {
		slog.Error("Failed to record prepared command delivery attempt in DB", "error", err, "command_id", msg.CommandID)
		_ = d.outboxRepo.UpdateOutboxRetry(ctx, msg.OutboxID, fmt.Sprintf("DB record delivery attempt error: %v", err), time.Now().Add(1*time.Second))
		return
	}

	// 3. Dispatch to target node via ClusterRouter or local Hub
	if d.router != nil && targetNodeID != "" {
		err = d.router.DispatchCommandRoute(ctx, msg.OrganizationID, msg.DeviceID, snap, targetNodeID, envData)
	} else {
		err = d.wsHub.DispatchToConnectionSnapshot(msg.OrganizationID, msg.DeviceID, snap, envData)
	}

	if err != nil {
		if latestCmd, getErr := d.cmdRepo.GetCommandByID(ctx, msg.CommandID); getErr == nil && latestCmd != nil && (latestCmd.Status == "ack" || latestCmd.Status == "executing" || latestCmd.Status == "succeeded") {
			slog.Info("Route receipt returned error/timeout but agent already fast-ACKed command. Preserving status.", "command_id", msg.CommandID, "status", latestCmd.Status)
			_ = d.cmdRepo.UpdateDeliveryAttemptStatus(ctx, msg.CommandID, attemptNo, "dispatched", "")
			_ = d.outboxRepo.UpdateOutboxDispatched(ctx, msg.OutboxID)
			return
		}

		_ = d.cmdRepo.UpdateDeliveryAttemptStatus(ctx, msg.CommandID, attemptNo, "failed", err.Error())
		if attemptNo >= d.maxAttempts {
			slog.Error("Outbox message permanently failed max retry limit", "outbox_id", msg.OutboxID, "device_id", msg.DeviceID, "attempts", attemptNo)
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

	// 4. Mark delivery attempt status 'dispatched' and outbox 'dispatched'
	err = d.cmdRepo.UpdateDeliveryAttemptStatus(ctx, msg.CommandID, attemptNo, "dispatched", "")
	if err != nil {
		slog.Error("Failed to update delivery attempt status to dispatched", "error", err, "command_id", msg.CommandID)
		_ = d.outboxRepo.UpdateOutboxRetry(ctx, msg.OutboxID, fmt.Sprintf("DB update delivery attempt status error: %v", err), time.Now().Add(1*time.Second))
		return
	}

	err = d.outboxRepo.UpdateOutboxDispatched(ctx, msg.OutboxID)
	if err != nil {
		slog.Error("Failed to update outbox dispatched status", "error", err, "outbox_id", msg.OutboxID)
		return
	}

	telemetry.GetMetrics().IncrCommandsDispatched(cmdRec.CommandType, "success")

	// 5. Broadcast DISPATCHED event across cluster
	if d.router != nil {
		d.router.BroadcastBrowserDeliveryEvent(msg.OrganizationID, msg.DeviceID, msg.CommandID, "dispatched", attemptNo)
	} else if d.browserHub != nil {
		d.browserHub.BroadcastCommandDelivery(msg.OrganizationID, msg.DeviceID, msg.CommandID, "dispatched", attemptNo)
	}
}

func (d *OutboxDispatcher) handleDispatchOffline(ctx context.Context, msg pgrepo.OutboxMessage, attemptNo int) {
	if attemptNo >= d.maxAttempts {
		_ = d.outboxRepo.UpdateOutboxFailed(ctx, msg.OutboxID, "Device offline after max attempts")
		_ = d.cmdRepo.UpdateCommandStatus(ctx, msg.CommandID, "failed", fmt.Sprintf("Device offline after %d attempts", d.maxAttempts), 0)
	} else {
		backoffSec := 1 << msg.AttemptCount
		_ = d.outboxRepo.UpdateOutboxRetry(ctx, msg.OutboxID, "Device agent not connected", time.Now().Add(time.Duration(backoffSec)*time.Second))
	}
}
