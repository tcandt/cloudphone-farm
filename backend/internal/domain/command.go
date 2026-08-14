package domain

import "time"

type DeviceCommand struct {
	CommandID      string    `json:"command_id"`
	DeviceID       string    `json:"device_id"`
	OrganizationID string    `json:"organization_id"`
	ActorID        string    `json:"actor_id"`
	ActorName      string    `json:"actor_name,omitempty"`
	CommandType    string    `json:"command_type"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}
