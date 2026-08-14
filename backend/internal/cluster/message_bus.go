package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RoutedEnvelope struct {
	MessageID      string          `json:"message_id"`
	SourceNodeID   string          `json:"source_node_id"`
	TargetNodeID   string          `json:"target_node_id"`
	OrganizationID string          `json:"organization_id"`
	DeviceID       string          `json:"device_id"`
	AgentID        string          `json:"agent_id"`
	ConnectionID   string          `json:"connection_id"`
	Generation     int64           `json:"generation"`
	Type           string          `json:"type"`
	Payload        json.RawMessage `json:"payload"`
	Timestamp      string          `json:"timestamp"`
}

type MessageBus struct {
	nodeID   string
	rdb      *redis.Client
	pubsub   *redis.PubSub
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

func NewMessageBus(nodeID string, rdb *redis.Client) *MessageBus {
	return &MessageBus{
		nodeID:   nodeID,
		rdb:      rdb,
		stopChan: make(chan struct{}),
	}
}

func nodeBusChannel(nodeID string) string {
	return fmt.Sprintf("pcp:v1:node-bus:%s", nodeID)
}

func deviceBusChannel(deviceID string) string {
	return fmt.Sprintf("pcp:v1:device-bus:%s", deviceID)
}

func (mb *MessageBus) PublishToNode(ctx context.Context, targetNodeID string, env *RoutedEnvelope) error {
	if mb.rdb == nil {
		return fmt.Errorf("redis client unavailable")
	}

	env.SourceNodeID = mb.nodeID
	env.TargetNodeID = targetNodeID
	env.Timestamp = time.Now().UTC().Format(time.RFC3339)

	bytes, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("failed to marshal routed envelope: %w", err)
	}

	ch := nodeBusChannel(targetNodeID)
	return mb.rdb.Publish(ctx, ch, bytes).Err()
}

func (mb *MessageBus) PublishToDevice(ctx context.Context, deviceID string, env *RoutedEnvelope) error {
	if mb.rdb == nil {
		return fmt.Errorf("redis client unavailable")
	}

	env.SourceNodeID = mb.nodeID
	env.Timestamp = time.Now().UTC().Format(time.RFC3339)

	bytes, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("failed to marshal routed envelope: %w", err)
	}

	ch := deviceBusChannel(deviceID)
	return mb.rdb.Publish(ctx, ch, bytes).Err()
}

func (mb *MessageBus) Subscribe(ctx context.Context, handler func(env *RoutedEnvelope)) error {
	if mb.rdb == nil {
		return fmt.Errorf("redis client unavailable")
	}

	mb.mu.Lock()
	ch := nodeBusChannel(mb.nodeID)
	mb.pubsub = mb.rdb.Subscribe(ctx, ch)
	mb.mu.Unlock()

	mb.wg.Add(1)
	go func() {
		defer mb.wg.Done()
		slog.Info("Subscribed backend node message bus channel", "channel", ch, "node_id", mb.nodeID)

		pubsubChan := mb.pubsub.Channel()
		for {
			select {
			case msg, ok := <-pubsubChan:
				if !ok {
					slog.Info("Message bus subscription channel closed", "node_id", mb.nodeID)
					return
				}
				if msg == nil {
					continue
				}

				var env RoutedEnvelope
				if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
					slog.Error("Failed to unmarshal routed envelope from cluster bus", "error", err, "payload", msg.Payload)
					continue
				}

				handler(&env)

			case <-mb.stopChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (mb *MessageBus) Close() {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	select {
	case <-mb.stopChan:
		return // Already closed
	default:
		close(mb.stopChan)
	}

	if mb.pubsub != nil {
		_ = mb.pubsub.Close()
	}

	mb.wg.Wait()
}
