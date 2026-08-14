package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
	"github.com/tcandt/cloudphone-farm/backend/internal/config"
	"github.com/tcandt/cloudphone-farm/backend/internal/repository/redis"
)

type AuthHandler struct {
	authService *auth.AuthService
	cfg         *config.Config
}

func NewAuthHandler(authService *auth.AuthService, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		cfg:         cfg,
	}
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserSessionDTO struct {
	SessionID        string   `json:"session_id"`
	UserID           string   `json:"user_id"`
	Email            string   `json:"email"`
	DisplayName      string   `json:"display_name"`
	OrganizationID   string   `json:"organization_id"`
	Role             string   `json:"role"`
	Permissions      []string `json:"permissions"`
	BalanceUSD       float64  `json:"balance_usd"`
	OrganizationName string   `json:"organization_name,omitempty"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":      "INVALID_JSON_BODY",
			"message":   "Malformed JSON request body",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	ipAddress := r.RemoteAddr
	userAgent := r.UserAgent()

	result, err := h.authService.Login(r.Context(), req.Email, req.Password, ipAddress, userAgent)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if errors.Is(err, auth.ErrInactiveMembership) {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "INACTIVE_MEMBERSHIP",
				"message":   "User account has no active organization membership",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		} else if errors.Is(err, redis.ErrRedisDown) {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "SERVICE_UNAVAILABLE",
				"message":   "Authoritative session store temporarily unavailable",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		} else if errors.Is(err, auth.ErrInvalidCredentials) {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "INVALID_CREDENTIALS",
				"message":   "Invalid email or password",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		} else {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"code":      "INTERNAL_SERVER_ERROR",
				"message":   "An unexpected authentication error occurred",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			})
		}
		return
	}

	// Configure cookie based on environment
	cookieName := h.cfg.SessionCookieName
	isSecure := h.cfg.SessionCookieSecure

	if !isSecure && h.cfg.AppEnv == "development" {
		cookieName = "pcp_session_dev"
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    result.RawToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   isSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   h.cfg.SessionTTLSeconds,
	})

	primaryRole := "viewer"
	if len(result.Principal.Roles) > 0 {
		primaryRole = result.Principal.Roles[0]
	}

	permissionList := make([]string, 0, len(result.Principal.Permissions))
	for p := range result.Principal.Permissions {
		permissionList = append(permissionList, p)
	}

	dto := UserSessionDTO{
		SessionID:      result.Principal.SessionID,
		UserID:         result.Principal.UserID,
		Email:          result.Principal.Email,
		DisplayName:    result.Principal.DisplayName,
		OrganizationID: result.Principal.OrganizationID,
		Role:           primaryRole,
		Permissions:    permissionList,
		BalanceUSD:     0.0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dto)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookieName := h.cfg.SessionCookieName
	cookie, err := r.Cookie(cookieName)
	if err != nil || cookie.Value == "" {
		if devCookie, devErr := r.Cookie("pcp_session_dev"); devErr == nil {
			cookie = devCookie
			cookieName = "pcp_session_dev"
		}
	}

	if cookie != nil && cookie.Value != "" {
		_ = h.authService.Logout(r.Context(), cookie.Value)
	}

	// Expire cookie
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.SessionCookieSecure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) Session(w http.ResponseWriter, r *http.Request) {
	principal, err := auth.GetPrincipal(r.Context())
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"code":      "UNAUTHENTICATED",
			"message":   "No active session principal found",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
		return
	}

	primaryRole := "viewer"
	if len(principal.Roles) > 0 {
		primaryRole = principal.Roles[0]
	}

	permissionList := make([]string, 0, len(principal.Permissions))
	for p := range principal.Permissions {
		permissionList = append(permissionList, p)
	}

	dto := UserSessionDTO{
		SessionID:      principal.SessionID,
		UserID:         principal.UserID,
		Email:          principal.Email,
		DisplayName:    principal.DisplayName,
		OrganizationID: principal.OrganizationID,
		Role:           primaryRole,
		Permissions:    permissionList,
		BalanceUSD:     0.0,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(dto)
}
