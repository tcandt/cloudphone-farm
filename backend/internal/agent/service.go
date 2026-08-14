package agent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	pgrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
	redisrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)

type AgentService struct {
	enrollRepo   *pgrepo.EnrollmentRepository
	presenceRepo *redisrepo.PresenceRepository
}

func NewAgentService(enrollRepo *pgrepo.EnrollmentRepository, presenceRepo *redisrepo.PresenceRepository) *AgentService {
	return &AgentService{
		enrollRepo:   enrollRepo,
		presenceRepo: presenceRepo,
	}
}

type CreateTokenRequest struct {
	BoundGroupID *string `json:"bound_group_id,omitempty"`
	TTLMinutes   int     `json:"ttl_minutes,omitempty"`
}

type TokenIssuedDTO struct {
	TokenID        string  `json:"token_id"`
	OrganizationID string  `json:"organization_id"`
	TokenCode      string  `json:"token_code"`
	CreatedBy      string  `json:"created_by"`
	ExpiresAt      string  `json:"expires_at"`
	BoundGroupID   *string `json:"bound_group_id,omitempty"`
}

type EnrollRequestDTO struct {
	TokenCode            string      `json:"token_code"`
	PublicKeyBytes       []byte      `json:"public_key_bytes"`
	ApkVersion           string      `json:"apk_version"`
	ProtocolVersion      string      `json:"protocol_version"`
	DeviceSerialNumber   string      `json:"device_serial_number"`
	DeviceModel          string      `json:"device_model"`
	DeviceAndroidVersion string      `json:"device_android_version"`
	DeviceDisplayName    string      `json:"device_display_name"`
	Capabilities         interface{} `json:"capabilities"`
}

type HeartbeatRequestDTO struct {
	ConnectionID string  `json:"connection_id"`
	Generation   int64   `json:"generation"`
	Sequence     int64   `json:"sequence"`
	Battery      int     `json:"battery"`
	Network      string  `json:"network"`
	CPUUsage     float64 `json:"cpu_usage"`
	RAMUsage     float64 `json:"ram_usage"`
	TemperatureC float64 `json:"temperature_c"`
}

func (s *AgentService) CreateEnrollmentToken(ctx context.Context, principal *auth.Principal, req CreateTokenRequest) (*TokenIssuedDTO, error) {
	if principal.OrganizationID == "" {
		return nil, errors.New("organization ID missing from principal")
	}

	ttl := 10 * time.Minute
	if req.TTLMinutes > 0 {
		ttl = time.Duration(req.TTLMinutes) * time.Minute
	}

	// Generate 16-byte raw token code
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, fmt.Errorf("failed to generate random token code: %w", err)
	}

	rawCode := fmt.Sprintf("PCP-%s-%s-%s",
		strings.ToUpper(hex.EncodeToString(b[:4])),
		strings.ToUpper(hex.EncodeToString(b[4:8])),
		strings.ToUpper(hex.EncodeToString(b[8:12])),
	)

	tokenHash := crypto.HashToken(rawCode)
	tokenID := fmt.Sprintf("ent_%s", uuid.New().String()[:12])
	now := time.Now()
	expiresAt := now.Add(ttl)

	rec := pgrepo.EnrollmentTokenRecord{
		TokenID:        tokenID,
		OrganizationID: principal.OrganizationID,
		TokenHash:      tokenHash,
		CreatedBy:      principal.UserID,
		BoundGroupID:   req.BoundGroupID,
		ExpiresAt:      expiresAt,
		CreatedAt:      now,
	}

	if err := s.enrollRepo.CreateToken(ctx, rec); err != nil {
		return nil, err
	}

	return &TokenIssuedDTO{
		TokenID:        tokenID,
		OrganizationID: principal.OrganizationID,
		TokenCode:      rawCode,
		CreatedBy:      principal.UserID,
		ExpiresAt:      expiresAt.UTC().Format(time.RFC3339),
		BoundGroupID:   req.BoundGroupID,
	}, nil
}

func (s *AgentService) ListEnrollmentTokens(ctx context.Context, orgID string) ([]pgrepo.EnrollmentTokenMetadata, error) {
	return s.enrollRepo.ListTokens(ctx, orgID)
}

func (s *AgentService) RevokeEnrollmentToken(ctx context.Context, orgID, tokenID string) error {
	return s.enrollRepo.RevokeToken(ctx, orgID, tokenID)
}

func (s *AgentService) EnrollAgent(ctx context.Context, req EnrollRequestDTO) (*pgrepo.EnrollmentResult, error) {
	if req.TokenCode == "" || len(req.PublicKeyBytes) == 0 || req.DeviceSerialNumber == "" {
		return nil, errors.New("missing required enrollment parameters")
	}

	// Validate Ed25519 public key size (must be exactly 32 bytes)
	if len(req.PublicKeyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("public key must be valid Ed25519 key of exactly %d bytes", ed25519.PublicKeySize)
	}

	fingerprint := crypto.ComputePublicKeyFingerprint(req.PublicKeyBytes)
	tokenHash := crypto.HashToken(req.TokenCode)

	var capsJSON []byte
	if req.Capabilities != nil {
		capsJSON, _ = json.Marshal(req.Capabilities)
	}
	if len(capsJSON) == 0 {
		defaultCaps := domain.DeviceCapabilities{}
		defaultCaps.Capture.Supported = true
		defaultCaps.Control.Supported = true
		defaultCaps.Control.Touch = true
		defaultCaps.Control.Swipe = true
		capsJSON, _ = json.Marshal(defaultCaps)
	}

	payload := pgrepo.AgentEnrollmentPayload{
		TokenCode:            req.TokenCode,
		TokenHash:            tokenHash,
		PublicKeyBytes:       req.PublicKeyBytes,
		PublicKeyFingerprint: fingerprint,
		ApkVersion:           req.ApkVersion,
		ProtocolVersion:      req.ProtocolVersion,
		DeviceSerialNumber:   req.DeviceSerialNumber,
		DeviceModel:          req.DeviceModel,
		DeviceAndroidVersion: req.DeviceAndroidVersion,
		DeviceDisplayName:    req.DeviceDisplayName,
		CapabilitiesJSON:     capsJSON,
	}

	return s.enrollRepo.ConsumeTokenAndRegisterDeviceAgent(ctx, payload)
}

func (s *AgentService) ProcessHeartbeat(ctx context.Context, agent *domain.DeviceAgent, req HeartbeatRequestDTO) error {
	presence := &domain.AgentPresencePayload{
		AgentID:      agent.AgentID,
		ConnectionID: req.ConnectionID,
		Generation:   req.Generation,
		Sequence:     req.Sequence,
		Health:       "healthy",
		LastSeenAt:   time.Now().UTC().Format(time.RFC3339),
	}

	// 1. Update 30s TTL Redis Presence (Atomic Lua CAS)
	if err := s.presenceRepo.UpdatePresence(ctx, agent.OrganizationID, agent.DeviceID, presence); err != nil {
		return err
	}

	// 2. Persist sampled telemetry to PostgreSQL device_heartbeats table
	_ = s.enrollRepo.RecordDeviceHeartbeat(ctx, agent.OrganizationID, agent.DeviceID, req.CPUUsage, req.RAMUsage, req.TemperatureC, req.Battery, req.Network)

	return nil
}
