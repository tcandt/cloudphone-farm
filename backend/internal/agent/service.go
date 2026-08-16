package agent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	pgrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
	redisrepo "github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)

type AgentService struct {
	enrollRepo    *pgrepo.EnrollmentRepository
	presenceRepo  *redisrepo.PresenceRepository
	agentConnRepo *redisrepo.AgentConnectionRepository
	rdb           *redis.Client
	broadcaster   ClusterRevocationBroadcaster
}

func NewAgentService(enrollRepo *pgrepo.EnrollmentRepository, presenceRepo *redisrepo.PresenceRepository, rdb *redis.Client) *AgentService {
	return &AgentService{
		enrollRepo:   enrollRepo,
		presenceRepo: presenceRepo,
		rdb:          rdb,
	}
}

func (s *AgentService) SetAgentConnectionRepository(connRepo *redisrepo.AgentConnectionRepository) {
	s.agentConnRepo = connRepo
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

type KeyProtectionDTO struct {
	Algorithm     string `json:"algorithm"`
	Provider      string `json:"provider"`
	SecurityLevel string `json:"security_level"`
}

type EnrollRequestDTO struct {
	TokenCode            string            `json:"token_code"`
	PublicKeyBytes       []byte            `json:"public_key_bytes"`
	ApkVersion           string            `json:"apk_version"`
	ProtocolVersion      string            `json:"protocol_version"`
	DeviceSerialNumber   string            `json:"device_serial_number"`
	DeviceModel          string            `json:"device_model"`
	DeviceAndroidVersion string            `json:"device_android_version"`
	DeviceDisplayName    string            `json:"device_display_name"`
	Capabilities         interface{}       `json:"capabilities"`
	KeyProtection        *KeyProtectionDTO `json:"key_protection,omitempty"`
}

type HeartbeatRequestDTO struct {
	ConnectionID  string            `json:"connection_id"`
	Generation    int64             `json:"generation"`
	Sequence      int64             `json:"sequence"`
	Battery       *int              `json:"battery,omitempty"`
	Network       *string           `json:"network,omitempty"`
	CPUUsage      *float64          `json:"cpu_usage,omitempty"`
	RAMUsage      *float64          `json:"ram_usage,omitempty"`
	TemperatureC  *float64          `json:"temperature_c,omitempty"`
	KeyProtection *KeyProtectionDTO `json:"key_protection,omitempty"`
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

type ClusterRevocationBroadcaster interface {
	BroadcastAgentRevocation(orgID, deviceID, agentID string)
}

func (s *AgentService) SetClusterBroadcaster(broadcaster ClusterRevocationBroadcaster) {
	s.broadcaster = broadcaster
}

func (s *AgentService) RevokeAgentCredential(ctx context.Context, orgID, agentID string) error {
	deviceID, err := s.enrollRepo.RevokeAgentCredential(ctx, orgID, agentID)
	if err != nil {
		return err
	}

	if s.presenceRepo != nil {
		_ = s.presenceRepo.RemovePresence(ctx, orgID, deviceID)
	}

	if s.broadcaster != nil {
		s.broadcaster.BroadcastAgentRevocation(orgID, deviceID, agentID)
	}

	return nil
}

func (s *AgentService) EnrollAgent(ctx context.Context, req EnrollRequestDTO) (*pgrepo.EnrollmentResult, error) {
	if req.TokenCode == "" || len(req.PublicKeyBytes) == 0 {
		return nil, errors.New("missing required enrollment parameters")
	}
	if req.DeviceSerialNumber == "" {
		req.DeviceSerialNumber = fmt.Sprintf("sn_%s", uuid.New().String()[:8])
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

	var keyProtJSON []byte
	if req.KeyProtection != nil {
		keyProtJSON, _ = json.Marshal(req.KeyProtection)
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
		KeyProtectionJSON:    keyProtJSON,
	}

	return s.enrollRepo.ConsumeTokenAndRegisterDeviceAgent(ctx, payload)
}

func (s *AgentService) ProcessHeartbeat(ctx context.Context, agent *domain.DeviceAgent, req HeartbeatRequestDTO) error {
	// 0. Fail-closed verification: heartbeat MUST match current active Redis WS owner tuple (connection_id, generation)
	if s.agentConnRepo == nil {
		return errors.New("heartbeat authority failure: agent connection repository is unavailable")
	}

	if req.ConnectionID == "" || req.Generation <= 0 {
		return errors.New("heartbeat owner mismatch: connection_id and generation must be provided")
	}

	owner, err := s.agentConnRepo.GetOwner(ctx, agent.OrganizationID, agent.DeviceID)
	if err != nil || owner == nil || owner.AgentID != agent.AgentID || owner.ConnectionID != req.ConnectionID || owner.Generation != req.Generation {
		slog.Warn("Heartbeat rejected: request does not match current authenticated WS owner",
			"device_id", agent.DeviceID,
			"agent_id", agent.AgentID,
			"req_conn_id", req.ConnectionID,
			"req_gen", req.Generation,
		)
		return errors.New("heartbeat owner mismatch: connection or generation does not match current authenticated WS owner")
	}

	presence := &domain.AgentPresencePayload{
		AgentID:      agent.AgentID,
		ConnectionID: req.ConnectionID,
		Generation:   req.Generation,
		Sequence:     req.Sequence,
		Health:       "healthy",
		LastSeenAt:   time.Now().UTC().Format(time.RFC3339),
	}

	// 1. Update 30s TTL Redis Presence (Atomic Lua CAS on every 10s heartbeat tick)
	if err := s.presenceRepo.UpdatePresence(ctx, agent.OrganizationID, agent.DeviceID, presence); err != nil {
		return err
	}

	// 2. Coalesced Telemetry Persistence to PostgreSQL (Once every 60s per device)
	shouldPersist := true
	if s.rdb != nil {
		coalesceKey := fmt.Sprintf("pcp:telemetry:persist:v1:%s:%s", agent.OrganizationID, agent.DeviceID)
		setOk, err := s.rdb.SetNX(ctx, coalesceKey, 1, 60*time.Second).Result()
		if err == nil && !setOk {
			shouldPersist = false // Telemetry write was already persisted within past 60s
		}
	}

	if shouldPersist {
		var keyProtJSON []byte
		if req.KeyProtection != nil {
			keyProtJSON, _ = json.Marshal(req.KeyProtection)
		}
		if err := s.enrollRepo.RecordDeviceHeartbeat(ctx, agent.OrganizationID, agent.DeviceID, req.CPUUsage, req.RAMUsage, req.TemperatureC, req.Battery, req.Network, keyProtJSON); err != nil {
			slog.Error("Failed to record PostgreSQL device heartbeat telemetry", "error", err, "device_id", agent.DeviceID)
		}
	}

	return nil
}
