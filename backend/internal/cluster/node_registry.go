package cluster

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrNodeNotFound = errors.New("target cluster node not found or heartbeat expired")
)

type NodeRecord struct {
	NodeID    string    `json:"node_id"`
	HostName  string    `json:"host_name"`
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type NodeRegistry struct {
	nodeID   string
	rdb      *redis.Client
	ttl      time.Duration
	stopChan chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

func NewNodeRegistry(nodeID string, rdb *redis.Client, ttl time.Duration) *NodeRegistry {
	if ttl <= 0 {
		ttl = 20 * time.Second
	}
	return &NodeRegistry{
		nodeID:   nodeID,
		rdb:      rdb,
		ttl:      ttl,
		stopChan: make(chan struct{}),
	}
}

func (nr *NodeRegistry) NodeID() string {
	return nr.nodeID
}

func nodeKey(nodeID string) string {
	return fmt.Sprintf("pcp:v1:node:%s", nodeID)
}

func (nr *NodeRegistry) Start(ctx context.Context) error {
	if nr.rdb == nil {
		return fmt.Errorf("redis client unavailable for node registry")
	}

	if err := nr.heartbeat(ctx); err != nil {
		return fmt.Errorf("initial node registration failed: %w", err)
	}

	nr.wg.Add(1)
	go func() {
		defer nr.wg.Done()
		tickerInterval := nr.ttl / 3
		if tickerInterval < 1*time.Second {
			tickerInterval = 1 * time.Second
		}
		ticker := time.NewTicker(tickerInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_ = nr.heartbeat(ctx)
			case <-nr.stopChan:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	slog.Info("Registered backend node in cluster registry", "node_id", nr.nodeID, "ttl_sec", nr.ttl.Seconds())
	return nil
}

func (nr *NodeRegistry) heartbeat(ctx context.Context) error {
	key := nodeKey(nr.nodeID)
	return nr.rdb.Set(ctx, key, time.Now().UTC().Format(time.RFC3339), nr.ttl).Err()
}

func (nr *NodeRegistry) Shutdown(ctx context.Context) {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	select {
	case <-nr.stopChan:
		return // Already stopped
	default:
		close(nr.stopChan)
	}

	nr.wg.Wait()

	if nr.rdb != nil {
		key := nodeKey(nr.nodeID)
		_ = nr.rdb.Del(ctx, key).Err()
		slog.Info("Unregistered backend node from cluster registry", "node_id", nr.nodeID)
	}
}

func (nr *NodeRegistry) IsNodeAlive(ctx context.Context, targetNodeID string) (bool, error) {
	if nr.rdb == nil {
		return false, fmt.Errorf("redis client unavailable")
	}

	key := nodeKey(targetNodeID)
	exists, err := nr.rdb.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}

	return exists > 0, nil
}
