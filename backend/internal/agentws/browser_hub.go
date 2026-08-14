package agentws

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"
)

type BrowserSubscriber struct {
	SubscriberID   string
	OrganizationID string
	DeviceID       string
	UserID         string
	Send           chan []byte
	Done           chan struct{}
	closeOnce      sync.Once
}

func NewBrowserSubscriber(subID, orgID, deviceID, userID string) *BrowserSubscriber {
	return &BrowserSubscriber{
		SubscriberID:   subID,
		OrganizationID: orgID,
		DeviceID:       deviceID,
		UserID:         userID,
		Send:           make(chan []byte, 64),
		Done:           make(chan struct{}),
	}
}

func (s *BrowserSubscriber) Close() {
	s.closeOnce.Do(func() {
		close(s.Done)
	})
}

type CommandStatusEvent struct {
	Type string                 `json:"type"`
	Data CommandStatusEventData `json:"data"`
}

type CommandStatusEventData struct {
	CommandID       string `json:"command_id"`
	DeviceID        string `json:"device_id"`
	ExecutionStatus string `json:"execution_status"`
	Sequence        int    `json:"sequence"`
	ErrorMessage    string `json:"error_message,omitempty"`
	OccurredAt      string `json:"occurred_at"`
}

type CommandDeliveryEvent struct {
	Type string                   `json:"type"`
	Data CommandDeliveryEventData `json:"data"`
}

type CommandDeliveryEventData struct {
	CommandID      string `json:"command_id"`
	DeviceID       string `json:"device_id"`
	DeliveryStatus string `json:"delivery_status"`
	AttemptCount   int    `json:"attempt_count"`
	DispatchedAt   string `json:"dispatched_at"`
}

type BrowserHub struct {
	subscribers map[string]map[string]*BrowserSubscriber // deviceKey -> subscriberID -> Subscriber
	mu          sync.RWMutex
}

func NewBrowserHub() *BrowserHub {
	return &BrowserHub{
		subscribers: make(map[string]map[string]*BrowserSubscriber),
	}
}

func (b *BrowserHub) Subscribe(sub *BrowserSubscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	key := DeviceKey(sub.OrganizationID, sub.DeviceID)
	if _, exists := b.subscribers[key]; !exists {
		b.subscribers[key] = make(map[string]*BrowserSubscriber)
	}
	b.subscribers[key][sub.SubscriberID] = sub
	slog.Info("Registered browser operator subscriber", "device_key", key, "user_id", sub.UserID, "subscriber_id", sub.SubscriberID)
}

func (b *BrowserHub) Unsubscribe(sub *BrowserSubscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()

	key := DeviceKey(sub.OrganizationID, sub.DeviceID)
	if subs, exists := b.subscribers[key]; exists {
		delete(subs, sub.SubscriberID)
		if len(subs) == 0 {
			delete(b.subscribers, key)
		}
		sub.Close()
		slog.Info("Unregistered browser operator subscriber", "device_key", key, "subscriber_id", sub.SubscriberID)
	}
}

func (b *BrowserHub) BroadcastCommandStatus(orgID, deviceID, commandID, executionStatus string, sequence int, errStr string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	key := DeviceKey(orgID, deviceID)
	subs, exists := b.subscribers[key]
	if !exists || len(subs) == 0 {
		return
	}

	event := CommandStatusEvent{
		Type: "command.status.changed",
		Data: CommandStatusEventData{
			CommandID:       commandID,
			DeviceID:        deviceID,
			ExecutionStatus: executionStatus,
			Sequence:        sequence,
			ErrorMessage:    errStr,
			OccurredAt:      time.Now().UTC().Format(time.RFC3339),
		},
	}

	bytes, err := json.Marshal(event)
	if err != nil {
		slog.Error("Failed to marshal command status event for browser fanout", "error", err)
		return
	}

	for _, sub := range subs {
		select {
		case sub.Send <- bytes:
		default:
			slog.Warn("Browser subscriber channel buffer full, skipping frame", "subscriber_id", sub.SubscriberID)
		}
	}
}

func (b *BrowserHub) BroadcastCommandDelivery(orgID, deviceID, commandID, deliveryStatus string, attemptCount int) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	key := DeviceKey(orgID, deviceID)
	subs, exists := b.subscribers[key]
	if !exists || len(subs) == 0 {
		return
	}

	event := CommandDeliveryEvent{
		Type: "command.delivery.changed",
		Data: CommandDeliveryEventData{
			CommandID:      commandID,
			DeviceID:       deviceID,
			DeliveryStatus: deliveryStatus,
			AttemptCount:   attemptCount,
			DispatchedAt:   time.Now().UTC().Format(time.RFC3339),
		},
	}

	bytes, err := json.Marshal(event)
	if err != nil {
		slog.Error("Failed to marshal command delivery event for browser fanout", "error", err)
		return
	}

	for _, sub := range subs {
		select {
		case sub.Send <- bytes:
		default:
			slog.Warn("Browser subscriber channel buffer full, skipping delivery frame", "subscriber_id", sub.SubscriberID)
		}
	}
}

func (b *BrowserHub) BroadcastRawMediaSignal(orgID, deviceID string, data []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	key := DeviceKey(orgID, deviceID)
	subs, exists := b.subscribers[key]
	if !exists || len(subs) == 0 {
		return
	}

	for _, sub := range subs {
		select {
		case sub.Send <- data:
		default:
			slog.Warn("Browser subscriber channel buffer full, skipping media signal frame", "subscriber_id", sub.SubscriberID)
		}
	}
}

