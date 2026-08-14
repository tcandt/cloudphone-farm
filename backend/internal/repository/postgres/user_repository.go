package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrMembershipNotFound = errors.New("no active organization membership found")
)

type UserRecord struct {
	UserID       string
	Email        string
	PasswordHash string
	DisplayName  string
	AvatarURL    *string
	CreatedAt    time.Time
}

type MembershipRecord struct {
	MembershipID   string
	OrganizationID string
	OrgName        string
	OrgSlug        string
	Status         string
}

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// GetUserByEmail fetches a user by normalized email
func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*UserRecord, error) {
	if r.pool == nil {
		return nil, errors.New("postgres connection pool uninitialized")
	}

	query := `
		SELECT user_id, email, password_hash, display_name, avatar_url, created_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
	`

	var u UserRecord
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&u.UserID, &u.Email, &u.PasswordHash, &u.DisplayName, &u.AvatarURL, &u.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to fetch user by email: %w", err)
	}

	return &u, nil
}

// GetActiveUserMemberships fetches active organization memberships for a user
func (r *UserRepository) GetActiveUserMemberships(ctx context.Context, userID string) ([]MembershipRecord, error) {
	if r.pool == nil {
		return nil, errors.New("postgres connection pool uninitialized")
	}

	query := `
		SELECT m.membership_id, m.organization_id, o.name, o.slug, m.status
		FROM organization_memberships m
		JOIN organizations o ON m.organization_id = o.organization_id
		WHERE m.user_id = $1 AND m.status = 'active'
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query memberships: %w", err)
	}
	defer rows.Close()

	var memberships []MembershipRecord
	for rows.Next() {
		var m MembershipRecord
		if err := rows.Scan(&m.MembershipID, &m.OrganizationID, &m.OrgName, &m.OrgSlug, &m.Status); err != nil {
			return nil, fmt.Errorf("failed to scan membership row: %w", err)
		}
		memberships = append(memberships, m)
	}

	if len(memberships) == 0 {
		return nil, ErrMembershipNotFound
	}

	return memberships, nil
}

// GetMembershipRolesAndPermissions fetches role codes and permission codes for a membership
func (r *UserRepository) GetMembershipRolesAndPermissions(ctx context.Context, membershipID string) ([]string, map[string]struct{}, error) {
	if r.pool == nil {
		return nil, nil, errors.New("postgres connection pool uninitialized")
	}

	// 1. Fetch roles
	roleQuery := `
		SELECT r.code
		FROM user_roles ur
		JOIN roles r ON ur.role_id = r.role_id
		WHERE ur.membership_id = $1
	`
	rows, err := r.pool.Query(ctx, roleQuery, membershipID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query roles: %w", err)
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err == nil {
			roles = append(roles, code)
		}
	}

	// 2. Fetch permissions via role_permissions
	permQuery := `
		SELECT DISTINCT rp.permission_code
		FROM user_roles ur
		JOIN role_permissions rp ON ur.role_id = rp.role_id
		WHERE ur.membership_id = $1
	`
	pRows, err := r.pool.Query(ctx, permQuery, membershipID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query permissions: %w", err)
	}
	defer pRows.Close()

	permissions := make(map[string]struct{})
	for pRows.Next() {
		var code string
		if err := pRows.Scan(&code); err == nil {
			permissions[code] = struct{}{}
		}
	}

	return roles, permissions, nil
}

// CreateSessionAudit creates a historical audit record in PostgreSQL sessions table
func (r *UserRepository) CreateSessionAudit(ctx context.Context, sessionID, userID, tokenHash, ip, userAgent string, expiresAt time.Time) error {
	if r.pool == nil {
		return nil // Non-fatal if DB is uninitialized during dev/test
	}

	query := `
		INSERT INTO sessions (session_id, user_id, token_hash, ip_address, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.pool.Exec(ctx, query, sessionID, userID, tokenHash, ip, userAgent, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to insert session audit record: %w", err)
	}

	return nil
}

// RevokeSessionAudit marks a session audit record as revoked in PostgreSQL
func (r *UserRepository) RevokeSessionAudit(ctx context.Context, tokenHash string) error {
	if r.pool == nil {
		return nil
	}

	query := `
		UPDATE sessions
		SET revoked_at = CURRENT_TIMESTAMP
		WHERE token_hash = $1 AND revoked_at IS NULL
	`
	_, err := r.pool.Exec(ctx, query, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to update session audit revocation: %w", err)
	}

	return nil
}
