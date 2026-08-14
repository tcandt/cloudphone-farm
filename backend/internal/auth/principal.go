package auth

import (
	"context"
	"errors"
)

type ctxKey string

const principalKey ctxKey = "auth_principal"

var ErrUnauthenticated = errors.New("unauthenticated: no valid session principal found in context")

// Principal represents the authoritative authenticated user session payload
type Principal struct {
	SessionID      string              `json:"session_id"`
	UserID         string              `json:"user_id"`
	Email          string              `json:"email"`
	DisplayName    string              `json:"display_name"`
	OrganizationID string              `json:"organization_id"`
	MembershipID   string              `json:"membership_id"`
	Roles          []string            `json:"roles"`
	Permissions    map[string]struct{} `json:"permissions"`
}

// HasPermission checks if the principal possesses a specific permission code
func (p *Principal) HasPermission(code string) bool {
	if p.Permissions == nil {
		return false
	}
	_, ok := p.Permissions[code]
	return ok
}

// WithPrincipal injects an authenticated Principal into the request context
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// GetPrincipal retrieves the authenticated Principal from context or returns an error
func GetPrincipal(ctx context.Context) (*Principal, error) {
	val := ctx.Value(principalKey)
	if val == nil {
		return nil, ErrUnauthenticated
	}
	p, ok := val.(*Principal)
	if !ok || p == nil {
		return nil, ErrUnauthenticated
	}
	return p, nil
}
