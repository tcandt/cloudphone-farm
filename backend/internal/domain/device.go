package domain

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrDeviceNotFound = errors.New("device not found or access denied")
)

type DeviceTelemetry struct {
	Battery      int       `json:"battery"`
	Network      string    `json:"network"`
	CPUUsage     float64   `json:"cpu_usage"`
	RAMUsage     float64   `json:"ram_usage"`
	TemperatureC float64   `json:"temperature_c"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type DeviceCapabilities struct {
	Capture struct {
		Supported bool     `json:"supported"`
		Codecs    []string `json:"codecs"`
		MaxWidth  int      `json:"max_width"`
		MaxHeight int      `json:"max_height"`
		MaxFPS    int      `json:"max_fps"`
	} `json:"capture"`
	Control struct {
		Supported        bool     `json:"supported"`
		Touch            bool     `json:"touch"`
		Swipe            bool     `json:"swipe"`
		GlobalActions    []string `json:"global_actions"`
		TextInput        string   `json:"text_input"`
		SensitiveActions bool     `json:"sensitive_actions"`
	} `json:"control"`
	Telemetry []string `json:"telemetry"`
	Transport []string `json:"transport"`
}

type Device struct {
	DeviceID        string             `json:"device_id"`
	OrganizationID  string             `json:"organization_id"`
	GroupID         *string            `json:"group_id,omitempty"`
	Name            string             `json:"name"`
	SerialNumber    string             `json:"serial_number"`
	Model           string             `json:"model"`
	PlatformVersion string             `json:"platform_version"`
	Status          string             `json:"status"`
	Capabilities    DeviceCapabilities `json:"capabilities"`
	Telemetry       *DeviceTelemetry   `json:"telemetry,omitempty"`
	LastSeenAt      *time.Time         `json:"last_seen_at,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

type DeviceListParams struct {
	Page    int
	Limit   int
	Status  string
	GroupID string
	Search  string
}

type DeviceListResult struct {
	Items []Device `json:"items"`
	Page  int      `json:"page"`
	Limit int      `json:"limit"`
	Total int      `json:"total"`
}

// DeriveRealtimeStatus calculates device presence status based on heartbeat threshold
func DeriveRealtimeStatus(lifecycleStatus string, lastSeen *time.Time, onlineThresholdSec, offlineThresholdSec int) string {
	// Respect immutable administrative lifecycle statuses
	if lifecycleStatus == "provisioning" || lifecycleStatus == "maintenance" || lifecycleStatus == "revoked" || lifecycleStatus == "retired" {
		return lifecycleStatus
	}

	if lastSeen == nil {
		return "offline"
	}

	elapsed := time.Since(*lastSeen).Seconds()
	if elapsed <= float64(onlineThresholdSec) {
		return "online"
	}
	if elapsed <= float64(offlineThresholdSec) {
		return "degraded"
	}

	return "offline"
}

func ParseCapabilitiesJSON(raw []byte) DeviceCapabilities {
	var caps DeviceCapabilities
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &caps)
	}
	return caps
}
