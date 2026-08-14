package domain

import (
	"errors"
	"time"
)

var (
	ErrControlAlreadyLeased = errors.New("device control is currently leased by another operator")
	ErrLeaseNotFound        = errors.New("control lease not found or expired")
	ErrLeaseNotOwned        = errors.New("control lease is owned by another operator")
	ErrInvalidLeaseToken    = errors.New("fencing token mismatch or stale control lease")
	ErrUnauthorizedCommand  = errors.New("command type requires elevated permissions")
)

type ControlLease struct {
	ControlLeaseID  string    `json:"control_lease_id"`
	DeviceID        string    `json:"device_id"`
	OrganizationID  string    `json:"organization_id"`
	UserID          string    `json:"user_id"`
	UserDisplayName string    `json:"user_display_name,omitempty"`
	FencingToken    int64     `json:"fencing_token"`
	AcquiredAt      time.Time `json:"acquired_at"`
	ExpiresAt       time.Time `json:"expires_at"`
	TTLSeconds      int       `json:"ttl_seconds"`
}
