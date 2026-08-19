package agentenrollment

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/google/uuid"
	"github.com/tcandt/cloudphone-farm/backend/internal/domain"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)

var (
	ErrInvalidRequest        = fmt.Errorf("invalid request")
	ErrUnauthorized          = fmt.Errorf("invalid enrollment credentials or challenge")
	ErrIdentityConflict      = fmt.Errorf("identity conflict")
	ErrQuotaExhausted        = fmt.Errorf("enrollment quota exhausted")
)

// AgentEnrollRequest matches the OpenAPI contract
type AgentEnrollRequest struct {
	EnrollmentToken  string                 `json:"enrollment_token"`
	ChallengeID      string                 `json:"challenge_id"`
	ClientInstanceID string                 `json:"client_instance_id"`
	PublicKey        string                 `json:"public_key"`
	Signature        string                 `json:"signature"`
	DeviceInfo       map[string]interface{} `json:"device_info"`
}

// AgentEnrollResponse matches the OpenAPI contract
type AgentEnrollResponse struct {
	AgentID  string `json:"agent_id"`
	DeviceID string `json:"device_id"`
}

type EnrollmentV2Service struct {
	repo           EnrollmentV2Repository
	challengeStore *ChallengeStore
}

func NewEnrollmentV2Service(repo EnrollmentV2Repository, challengeStore *ChallengeStore) *EnrollmentV2Service {
	return &EnrollmentV2Service{
		repo:           repo,
		challengeStore: challengeStore,
	}
}

func (s *EnrollmentV2Service) GenerateChallenge(ctx context.Context, token, clientInstanceID, publicKeyBase64 string) (string, string, error) {
	// 1. Hash raw token
	tokenHash := crypto.HashToken(token)

	// 2. Resolve V2 key
	key, err := s.repo.ResolveKey(ctx, tokenHash)
	if err != nil {
		slog.Error("failed to resolve key", "err", err)
		return "", "", ErrUnauthorized
	}
	if key == nil {
		return "", "", ErrUnauthorized
	}

	if key.RevokedAt != nil || (key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now())) {
		return "", "", ErrUnauthorized
	}

	// 3. Parse and validate P-256 SPKI public key
	pubKeyBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return "", "", ErrUnauthorized
	}
	parsedKey, err := x509.ParsePKIXPublicKey(pubKeyBytes)
	if err != nil {
		return "", "", ErrUnauthorized
	}
	ecdsaKey, ok := parsedKey.(*ecdsa.PublicKey)
	if !ok || ecdsaKey.Curve.Params().Name != "P-256" {
		return "", "", ErrUnauthorized
	}

	// 4. Generate fingerprint (SHA-256 hex exactly 64 chars)
	fpBytes := sha256.Sum256(pubKeyBytes)
	fingerprint := hex.EncodeToString(fpBytes[:])

	// 5. Generate 32-byte CSPRNG nonce
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate nonce: %w", err)
	}
	nonceB64url := base64.RawURLEncoding.EncodeToString(nonceBytes)

	// Generate challenge ID
	challengeID := "chl_" + uuid.New().String()

	// 6. Save challenge context to Redis
	challengeCtx := ChallengeContext{
		KeyID:                key.KeyID,
		OrganizationID:       key.OrganizationID,
		ClientInstanceID:     clientInstanceID,
		PublicKeyFingerprint: fingerprint,
		Nonce:                nonceB64url,
	}

	if err := s.challengeStore.SaveChallenge(ctx, challengeID, challengeCtx); err != nil {
		slog.Error("failed to save challenge", "err", err)
		return "", "", fmt.Errorf("failed to create challenge")
	}

	return challengeID, nonceB64url, nil
}

type ecdsaSignature struct {
	R, S *big.Int
}

func (s *EnrollmentV2Service) EnrollAgent(ctx context.Context, req AgentEnrollRequest) (*AgentEnrollResponse, bool, error) {
	// 1. Atomically GETDEL challenge
	challengeCtx, err := s.challengeStore.ConsumeChallenge(ctx, req.ChallengeID)
	if err != nil {
		slog.Warn("challenge not found or expired", "challenge_id", req.ChallengeID, "err", err)
		return nil, false, ErrUnauthorized
	}

	// Double check identity match
	if challengeCtx.ClientInstanceID != req.ClientInstanceID {
		return nil, false, ErrUnauthorized
	}

	pubKeyBytes, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil {
		return nil, false, ErrUnauthorized
	}
	fpBytes := sha256.Sum256(pubKeyBytes)
	if hex.EncodeToString(fpBytes[:]) != challengeCtx.PublicKeyFingerprint {
		return nil, false, ErrUnauthorized
	}

	// 2. Verify ECDSA signature over raw nonce
	nonceRaw, err := base64.RawURLEncoding.DecodeString(challengeCtx.Nonce)
	if err != nil {
		return nil, false, ErrUnauthorized
	}
	sigBytes, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil {
		return nil, false, ErrUnauthorized
	}

	var sig ecdsaSignature
	if _, err := asn1.Unmarshal(sigBytes, &sig); err != nil {
		return nil, false, ErrUnauthorized
	}

	parsedKey, err := x509.ParsePKIXPublicKey(pubKeyBytes)
	if err != nil {
		return nil, false, ErrUnauthorized
	}
	ecdsaKey, ok := parsedKey.(*ecdsa.PublicKey)
	if !ok || ecdsaKey.Curve.Params().Name != "P-256" {
		return nil, false, ErrUnauthorized
	}

	hash := sha256.Sum256(nonceRaw)
	if !ecdsa.Verify(ecdsaKey, hash[:], sig.R, sig.S) {
		slog.Warn("signature verification failed")
		return nil, false, ErrUnauthorized
	}

	// 3. PostgreSQL Transaction
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("failed to begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	tokenHash := crypto.HashToken(req.EnrollmentToken)

	// 4. SELECT FOR UPDATE
	key, err := tx.RevalidateKey(ctx, challengeCtx.OrganizationID, challengeCtx.KeyID, tokenHash)
	if err != nil {
		slog.Error("failed to revalidate key", "err", err)
		return nil, false, ErrUnauthorized
	}
	if key == nil {
		return nil, false, ErrUnauthorized
	}

	// 5. Revalidate revoked/expired
	if key.RevokedAt != nil || (key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now())) {
		return nil, false, ErrUnauthorized
	}

	// 6. Idempotency Check BEFORE Quota
	idem, err := tx.CheckIdempotency(ctx, challengeCtx.OrganizationID, challengeCtx.ClientInstanceID)
	if err != nil {
		slog.Error("failed to check idempotency", "err", err)
		return nil, false, fmt.Errorf("internal error")
	}

	if idem.Exists {
		if idem.Fingerprint == challengeCtx.PublicKeyFingerprint {
			// Idempotent 200 OK - No quota consumption
			tx.UpdateKeyLastUsedAt(ctx, challengeCtx.OrganizationID, challengeCtx.KeyID)
			tx.Commit(ctx)
			return &AgentEnrollResponse{
				AgentID:  idem.AgentID,
				DeviceID: idem.DeviceID,
			}, false, nil
		}
		// Fingerprint mismatch
		return nil, false, ErrIdentityConflict
	}

	// 7. NEW Identity -> Quota Enforcement
	if key.MaxBindings != nil {
		count, err := tx.CountActiveBindings(ctx, challengeCtx.OrganizationID, challengeCtx.KeyID)
		if err != nil {
			slog.Error("failed to count bindings", "err", err)
			return nil, false, fmt.Errorf("internal error")
		}
		if count >= *key.MaxBindings {
			return nil, false, ErrQuotaExhausted
		}
	}

	// 8. Create Device, DeviceAgent, Binding
	deviceID := "dev_" + uuid.New().String()
	agentID := "agt_" + uuid.New().String()
	bindingID := "akb_" + uuid.New().String()

	device := &domain.Device{
		DeviceID:        deviceID,
		OrganizationID:  challengeCtx.OrganizationID,
		Name:            fmt.Sprintf("Device %s", deviceID[:8]),
		SerialNumber:    "unknown",
		Model:           "unknown",
		PlatformVersion: "unknown",
		Status:          "provisioning",
	}

	if sn, ok := req.DeviceInfo["serial_number"].(string); ok && sn != "" {
		device.SerialNumber = sn
	}
	if mod, ok := req.DeviceInfo["model"].(string); ok && mod != "" {
		device.Model = mod
	}
	if pv, ok := req.DeviceInfo["android_version"].(string); ok && pv != "" {
		device.PlatformVersion = pv
	}

	clientInstID := challengeCtx.ClientInstanceID
	agent := &domain.DeviceAgent{
		AgentID:              agentID,
		OrganizationID:       challengeCtx.OrganizationID,
		DeviceID:             deviceID,
		ClientInstanceID:     &clientInstID,
		PublicKeyFingerprint: challengeCtx.PublicKeyFingerprint,
		ApkVersion:           "1.0.0",
		Status:               "active",
	}

	if av, ok := req.DeviceInfo["agent_version"].(string); ok && av != "" {
		agent.ApkVersion = av
	}

	binding := &domain.AgentKeyBinding{
		BindingID:            bindingID,
		OrganizationID:       challengeCtx.OrganizationID,
		KeyID:                challengeCtx.KeyID,
		DeviceID:             deviceID,
		AgentID:              agentID,
		PublicKeyFingerprint: challengeCtx.PublicKeyFingerprint,
		BoundAt:              time.Now().UTC(),
	}

	if err := tx.CreateBinding(ctx, device, agent, binding); err != nil {
		slog.Error("failed to create binding", "err", err)
		return nil, false, fmt.Errorf("internal error")
	}

	if err := tx.UpdateKeyLastUsedAt(ctx, challengeCtx.OrganizationID, challengeCtx.KeyID); err != nil {
		slog.Error("failed to update last used", "err", err)
		return nil, false, fmt.Errorf("internal error")
	}

	// 9. Commit
	if err := tx.Commit(ctx); err != nil {
		slog.Error("failed to commit tx", "err", err)
		return nil, false, fmt.Errorf("internal error")
	}

	return &AgentEnrollResponse{
		AgentID:  agentID,
		DeviceID: deviceID,
	}, true, nil
}
