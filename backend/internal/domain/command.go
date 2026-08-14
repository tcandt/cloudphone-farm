package domain

import (
	"errors"
	"time"
)

var (
	ErrIdempotencyKeyRequired = errors.New("idempotency key is required for command dispatch")
	ErrIdempotencyConflict    = errors.New("idempotency key reused with different command parameters")
)

type DeviceCommand struct {
	CommandID      string                 `json:"command_id"`
	DeviceID       string                 `json:"device_id"`
	OrganizationID string                 `json:"organization_id"`
	ActorID        string                 `json:"actor_id"`
	ActorName      string                 `json:"actor_name,omitempty"`
	CommandType    string                 `json:"command_type"`
	Payload        map[string]interface{} `json:"payload,omitempty"`
	Status         string                 `json:"status"`
	CreatedAt      time.Time              `json:"created_at"`
}
