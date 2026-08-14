package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/tcandt/cloudphone-farm/backend/internal/agentws"
	redispkg "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
)

var (
	ErrRouteTimeout  = errors.New("cross-node command route receipt timeout")
	ErrRouteRejected = errors.New("target node rejected cross-node command route request (fencing mismatch or socket disconnected)")
)

type RouteReceipt struct {
	MessageID string `json:"message_id"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

type ClusterRouter struct {
	nodeID        string
	bus           *MessageBus
	agentConnRepo *redispkg.AgentConnectionRepository
	mediaRepo     *redispkg.MediaSessionRepository
	wsHub         *agentws.Hub
	browserHub    *agentws.BrowserHub

	receiptChans map[string]chan RouteReceipt
	receiptMu    sync.Mutex
}

func NewClusterRouter(
	nodeID string,
	bus *MessageBus,
	agentConnRepo *redispkg.AgentConnectionRepository,
	mediaRepo *redispkg.MediaSessionRepository,
	wsHub *agentws.Hub,
	browserHub *agentws.BrowserHub,
) *ClusterRouter {
	return &ClusterRouter{
		nodeID:        nodeID,
		bus:           bus,
		agentConnRepo: agentConnRepo,
		mediaRepo:     mediaRepo,
		wsHub:         wsHub,
		browserHub:    browserHub,
		receiptChans:  make(map[string]chan RouteReceipt),
	}
}

func (cr *ClusterRouter) Start(ctx context.Context) error {
	return cr.bus.Subscribe(ctx, cr.handleIncomingEnvelope)
}

func (cr *ClusterRouter) handleIncomingEnvelope(env *RoutedEnvelope) {
	switch env.Type {
	case "command.route.request":
		cr.handleCommandRouteRequest(env)
	case "command.route.receipt":
		cr.handleCommandRouteReceipt(env)
	case "browser.event.status":
		cr.handleBrowserEventStatus(env)
	case "browser.event.delivery":
		cr.handleBrowserEventDelivery(env)
	case "media.signal.to_agent":
		cr.handleMediaSignalToAgent(env)
	case "media.signal.to_browser":
		cr.handleMediaSignalToBrowser(env)
	default:
		slog.Warn("Unknown envelope type received on cluster bus", "type", env.Type, "msg_id", env.MessageID)
	}
}

func (cr *ClusterRouter) handleCommandRouteRequest(env *RoutedEnvelope) {
	snap := agentws.ConnectionSnapshot{
		AgentID:      env.AgentID,
		ConnectionID: env.ConnectionID,
		Generation:   env.Generation,
	}

	err := cr.wsHub.DispatchToConnectionSnapshot(env.OrganizationID, env.DeviceID, snap, env.Payload)

	receipt := RouteReceipt{
		MessageID: env.MessageID,
		Success:   err == nil,
	}
	if err != nil {
		receipt.Error = err.Error()
	}

	receiptBytes, _ := json.Marshal(receipt)
	receiptEnv := &RoutedEnvelope{
		MessageID:      env.MessageID,
		SourceNodeID:   cr.nodeID,
		TargetNodeID:   env.SourceNodeID,
		OrganizationID: env.OrganizationID,
		DeviceID:       env.DeviceID,
		Type:           "command.route.receipt",
		Payload:        receiptBytes,
	}

	_ = cr.bus.PublishToNode(context.Background(), env.SourceNodeID, receiptEnv)
}

func (cr *ClusterRouter) handleCommandRouteReceipt(env *RoutedEnvelope) {
	var receipt RouteReceipt
	if err := json.Unmarshal(env.Payload, &receipt); err != nil {
		return
	}

	cr.receiptMu.Lock()
	ch, exists := cr.receiptChans[receipt.MessageID]
	cr.receiptMu.Unlock()

	if exists && ch != nil {
		select {
		case ch <- receipt:
		default:
		}
	}
}

func (cr *ClusterRouter) handleBrowserEventStatus(env *RoutedEnvelope) {
	var evt agentws.CommandStatusEvent
	if err := json.Unmarshal(env.Payload, &evt); err != nil {
		return
	}

	if cr.browserHub != nil {
		cr.browserHub.BroadcastCommandStatus(
			env.OrganizationID,
			env.DeviceID,
			evt.Data.CommandID,
			evt.Data.ExecutionStatus,
			evt.Data.Sequence,
			evt.Data.ErrorMessage,
		)
	}
}

func (cr *ClusterRouter) handleBrowserEventDelivery(env *RoutedEnvelope) {
	var evt agentws.CommandDeliveryEvent
	if err := json.Unmarshal(env.Payload, &evt); err != nil {
		return
	}

	if cr.browserHub != nil {
		cr.browserHub.BroadcastCommandDelivery(
			env.OrganizationID,
			env.DeviceID,
			evt.Data.CommandID,
			evt.Data.DeliveryStatus,
			evt.Data.AttemptCount,
		)
	}
}

func (cr *ClusterRouter) handleMediaSignalToAgent(env *RoutedEnvelope) {
	snap := agentws.ConnectionSnapshot{
		AgentID:      env.AgentID,
		ConnectionID: env.ConnectionID,
		Generation:   env.Generation,
	}
	_ = cr.wsHub.DispatchToConnectionSnapshot(env.OrganizationID, env.DeviceID, snap, env.Payload)
}

func (cr *ClusterRouter) handleMediaSignalToBrowser(env *RoutedEnvelope) {
	if cr.browserHub != nil {
		cr.browserHub.BroadcastRawMediaSignal(env.OrganizationID, env.DeviceID, env.Payload)
	}
}

// DispatchCommandRoute sends command payload to owner node (or dispatches locally if owner is local) and waits for route receipt.
func (cr *ClusterRouter) DispatchCommandRoute(
	ctx context.Context,
	orgID, deviceID string,
	snap agentws.ConnectionSnapshot,
	ownerNodeID string,
	payload []byte,
) error {
	if ownerNodeID == cr.nodeID {
		return cr.wsHub.DispatchToConnectionSnapshot(orgID, deviceID, snap, payload)
	}

	msgID := fmt.Sprintf("route_%s", uuid.New().String()[:8])
	receiptCh := make(chan RouteReceipt, 1)

	cr.receiptMu.Lock()
	cr.receiptChans[msgID] = receiptCh
	cr.receiptMu.Unlock()

	defer func() {
		cr.receiptMu.Lock()
		delete(cr.receiptChans, msgID)
		cr.receiptMu.Unlock()
	}()

	env := &RoutedEnvelope{
		MessageID:      msgID,
		SourceNodeID:   cr.nodeID,
		TargetNodeID:   ownerNodeID,
		OrganizationID: orgID,
		DeviceID:       deviceID,
		AgentID:        snap.AgentID,
		ConnectionID:   snap.ConnectionID,
		Generation:     snap.Generation,
		Type:           "command.route.request",
		Payload:        payload,
	}

	if err := cr.bus.PublishToNode(ctx, ownerNodeID, env); err != nil {
		return fmt.Errorf("failed to publish command route request to target node %s: %w", ownerNodeID, err)
	}

	select {
	case receipt := <-receiptCh:
		if receipt.Success {
			return nil
		}
		return fmt.Errorf("%w: %s", ErrRouteRejected, receipt.Error)
	case <-time.After(2 * time.Second):
		return ErrRouteTimeout
	case <-ctx.Done():
		return ctx.Err()
	}
}

// BroadcastBrowserStatusEvent publishes command status event across cluster nodes to reach all browser subscribers.
func (cr *ClusterRouter) BroadcastBrowserStatusEvent(orgID, deviceID, commandID, executionStatus string, sequence int, errStr string) {
	evt := agentws.CommandStatusEvent{
		Type: "command.status.changed",
		Data: agentws.CommandStatusEventData{
			CommandID:       commandID,
			DeviceID:        deviceID,
			ExecutionStatus: executionStatus,
			Sequence:        sequence,
			ErrorMessage:    errStr,
			OccurredAt:      time.Now().UTC().Format(time.RFC3339),
		},
	}
	bytes, _ := json.Marshal(evt)

	env := &RoutedEnvelope{
		MessageID:      fmt.Sprintf("evt_status_%s", uuid.New().String()[:8]),
		OrganizationID: orgID,
		DeviceID:       deviceID,
		Type:           "browser.event.status",
		Payload:        bytes,
	}

	_ = cr.bus.PublishToDevice(context.Background(), deviceID, env)
}

// BroadcastBrowserDeliveryEvent publishes command delivery event across cluster nodes to reach all browser subscribers.
func (cr *ClusterRouter) BroadcastBrowserDeliveryEvent(orgID, deviceID, commandID, deliveryStatus string, attemptCount int) {
	evt := agentws.CommandDeliveryEvent{
		Type: "command.delivery.changed",
		Data: agentws.CommandDeliveryEventData{
			CommandID:      commandID,
			DeviceID:       deviceID,
			DeliveryStatus: deliveryStatus,
			AttemptCount:   attemptCount,
			DispatchedAt:   time.Now().UTC().Format(time.RFC3339),
		},
	}
	bytes, _ := json.Marshal(evt)

	env := &RoutedEnvelope{
		MessageID:      fmt.Sprintf("evt_deliv_%s", uuid.New().String()[:8]),
		OrganizationID: orgID,
		DeviceID:       deviceID,
		Type:           "browser.event.delivery",
		Payload:        bytes,
	}

	_ = cr.bus.PublishToDevice(context.Background(), deviceID, env)
}
