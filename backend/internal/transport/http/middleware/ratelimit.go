package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"github.com/tcandt/cloudphone-farm/backend/internal/auth"
)

type RateLimitScope string

const (
	ScopeLogin      RateLimitScope = "login"
	ScopeEnrollment RateLimitScope = "enrollment"
	ScopeRestAPI    RateLimitScope = "rest_api"
	ScopeCommand    RateLimitScope = "command"
	ScopeWSUpgrade  RateLimitScope = "ws_upgrade"
)

type RateLimiter struct {
	rdb       *redis.Client
	appEnv    string
	isProd    bool
}

func NewRateLimiter(rdb *redis.Client, appEnv string) *RateLimiter {
	return &RateLimiter{
		rdb:    rdb,
		appEnv: appEnv,
		isProd: strings.ToLower(appEnv) == "production",
	}
}

// Token Bucket Lua script
const tokenBucketScript = `
local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local fill_rate = tonumber(ARGV[2]) -- tokens per second
local now_sec = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])
local ttl_sec = tonumber(ARGV[5])

local data = redis.call('GET', key)
local tokens = capacity
local last_update = now_sec

if data then
    local parsed = cjson.decode(data)
    tokens = tonumber(parsed.tokens)
    last_update = tonumber(parsed.last_update)
    
    local delta = math.max(0, now_sec - last_update)
    tokens = math.min(capacity, tokens + (delta * fill_rate))
end

if tokens >= requested then
    tokens = tokens - requested
    local payload = cjson.encode({tokens = tokens, last_update = now_sec})
    redis.call('SET', key, payload, 'EX', ttl_sec)
    return 1
else
    local payload = cjson.encode({tokens = tokens, last_update = last_update})
    redis.call('SET', key, payload, 'EX', ttl_sec)
    return 0
end
`

// ExtractClientIP extracts client IP securely.
// Only trusts X-Real-IP if the remote address is within TRUSTED_PROXY_CIDR environment setting.
func ExtractClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	trustedCIDR := os.Getenv("TRUSTED_PROXY_CIDR")
	if trustedCIDR != "" {
		_, cidrNet, err := net.ParseCIDR(trustedCIDR)
		if err == nil && cidrNet.Contains(net.ParseIP(host)) {
			if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
				return realIP
			}
		}
	}

	return host
}

func (rl *RateLimiter) CheckLimit(ctx context.Context, scope RateLimitScope, identifier string, capacity int, fillRate float64) (bool, error) {
	if rl.rdb == nil {
		if rl.isProd {
			return false, fmt.Errorf("redis rate limiter unavailable in production mode")
		}
		return true, nil
	}

	key := fmt.Sprintf("pcp:v1:ratelimit:%s:%s", scope, identifier)
	nowSec := time.Now().Unix()
	ttlSec := 3600

	res, err := rl.rdb.Eval(ctx, tokenBucketScript, []string{key}, capacity, fillRate, nowSec, 1, ttlSec).Int64()
	if err != nil {
		if rl.isProd {
			return false, fmt.Errorf("rate limit evaluation error in production: %w", err)
		}
		slog.Warn("Rate limiter evaluation error, allowing request in non-prod", "error", err)
		return true, nil
	}

	return res == 1, nil
}

func (rl *RateLimiter) LimitMiddleware(scope RateLimitScope, capacity int, fillRate float64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ExtractClientIP(r)
			var identifier string

			// Extract authoritative principal from request context
			var orgID, userID string
			if principal, _ := auth.GetPrincipal(r.Context()); principal != nil {
				orgID = principal.OrganizationID
				userID = principal.UserID
			}

			deviceID := chi.URLParam(r, "id")

			switch scope {
			case ScopeLogin, ScopeEnrollment:
				identifier = ip
			case ScopeWSUpgrade:
				if deviceID != "" {
					identifier = fmt.Sprintf("%s:%s", deviceID, ip)
				} else {
					identifier = ip
				}
			case ScopeRestAPI:
				if orgID == "" {
					orgID = "anon"
				}
				identifier = fmt.Sprintf("%s:%s", orgID, ip)
			case ScopeCommand:
				if orgID == "" {
					orgID = "anon"
				}
				if userID == "" {
					userID = "anon"
				}
				if deviceID == "" {
					deviceID = "unknown"
				}
				identifier = fmt.Sprintf("%s:%s:%s", orgID, userID, deviceID)
			default:
				identifier = ip
			}

			allowed, err := rl.CheckLimit(r.Context(), scope, identifier, capacity, fillRate)
			if err != nil || !allowed {
				if err != nil {
					slog.Error("Rate limit check failed", "scope", scope, "identifier", identifier, "error", err)
				} else {
					slog.Warn("Rate limit exceeded", "scope", scope, "identifier", identifier)
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"code":    "RATE_LIMIT_EXCEEDED",
					"message": "Too many requests. Please slow down.",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
