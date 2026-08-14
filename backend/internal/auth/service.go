package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tcandt/cloudphone-farm/backend/internal/repository/postgres"
	"github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
	"github.com/tcandt/cloudphone-farm/backend/pkg/crypto"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrInactiveMembership = errors.New("user has no active organization membership")
)

type AuthService struct {
	userRepo    *postgres.UserRepository
	sessionRepo *redis.SessionRepository
	ttl         time.Duration
}

func NewAuthService(userRepo *postgres.UserRepository, sessionRepo *redis.SessionRepository, ttl time.Duration) *AuthService {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	return &AuthService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		ttl:         ttl,
	}
}

type LoginResult struct {
	RawToken  string     `json:"raw_token"`
	Principal *Principal `json:"principal"`
}

// Login performs authoritative Argon2id authentication and issues a 256-bit opaque token
func (s *AuthService) Login(ctx context.Context, email, password, ipAddress, userAgent string) (*LoginResult, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	// 1. Load user from PostgreSQL
	user, err := s.userRepo.GetUserByEmail(ctx, normalizedEmail)
	if err != nil {
		if errors.Is(err, postgres.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("failed to fetch user: %w", err)
	}

	// 2. Verify Argon2id password (constant-time, generic error response)
	match, err := crypto.VerifyPassword(password, user.PasswordHash)
	if err != nil || !match {
		return nil, ErrInvalidCredentials
	}

	// 3. Load active memberships
	memberships, err := s.userRepo.GetActiveUserMemberships(ctx, user.UserID)
	if err != nil || len(memberships) == 0 {
		return nil, ErrInactiveMembership
	}

	// Default to first active membership
	activeMembership := memberships[0]

	// 4. Resolve roles & permissions
	roles, permissions, err := s.userRepo.GetMembershipRolesAndPermissions(ctx, activeMembership.MembershipID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve roles and permissions: %w", err)
	}

	// 5. Generate 256-bit opaque random token and compute SHA-256 hash
	rawToken, err := crypto.GenerateOpaqueToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate session token: %w", err)
	}

	tokenHash := crypto.HashToken(rawToken)
	sessionID := fmt.Sprintf("ses_%s", uuid.New().String()[:12])

	principal := &Principal{
		SessionID:      sessionID,
		UserID:         user.UserID,
		Email:          user.Email,
		DisplayName:    user.DisplayName,
		OrganizationID: activeMembership.OrganizationID,
		MembershipID:   activeMembership.MembershipID,
		Roles:          roles,
		Permissions:    permissions,
	}

	sessionData := &redis.SessionData{
		SessionID:      principal.SessionID,
		UserID:         principal.UserID,
		Email:          principal.Email,
		DisplayName:    principal.DisplayName,
		OrganizationID: principal.OrganizationID,
		MembershipID:   principal.MembershipID,
		Roles:          principal.Roles,
		Permissions:    principal.Permissions,
	}

	// 6. Store in Redis authoritative session store
	if err := s.sessionRepo.CreateSession(ctx, tokenHash, sessionData, s.ttl); err != nil {
		return nil, fmt.Errorf("failed to store authoritative session in Redis: %w", err)
	}

	// 7. Audit log in PostgreSQL
	expiresAt := time.Now().Add(s.ttl)
	_ = s.userRepo.CreateSessionAudit(ctx, sessionID, user.UserID, tokenHash, ipAddress, userAgent, expiresAt)

	return &LoginResult{
		RawToken:  rawToken,
		Principal: principal,
	}, nil
}

// Logout invalidates a session in Redis and marks PostgreSQL session audit as revoked
func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}

	tokenHash := crypto.HashToken(rawToken)

	// 1. Delete from Redis authoritative store
	if err := s.sessionRepo.DeleteSession(ctx, tokenHash); err != nil {
		// Log warning, continue to revoke audit
	}

	// 2. Mark revoked in PostgreSQL audit
	_ = s.userRepo.RevokeSessionAudit(ctx, tokenHash)

	return nil
}

// GetSessionByToken retrieves the authenticated principal using the raw session token
func (s *AuthService) GetSessionByToken(ctx context.Context, rawToken string) (*Principal, error) {
	if rawToken == "" {
		return nil, ErrUnauthenticated
	}

	tokenHash := crypto.HashToken(rawToken)
	sessionData, err := s.sessionRepo.GetSession(ctx, tokenHash)
	if err != nil {
		return nil, ErrUnauthenticated
	}

	return &Principal{
		SessionID:      sessionData.SessionID,
		UserID:         sessionData.UserID,
		Email:          sessionData.Email,
		DisplayName:    sessionData.DisplayName,
		OrganizationID: sessionData.OrganizationID,
		MembershipID:   sessionData.MembershipID,
		Roles:          sessionData.Roles,
		Permissions:    sessionData.Permissions,
	}, nil
}
