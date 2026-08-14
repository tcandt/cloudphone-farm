package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimitScope string

const (
	ScopeLogin      RateLimitScope = "login"
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

// ExtractClientIP extracts the real client IP, inspecting RemoteAddr or trusted X-Real-IP
func ExtractClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	// Only trust X-Real-IP if configured or present
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}

	return host
}

func (rl *RateLimiter) CheckLimit(ctx context.Context, scope RateLimitScope, identifier string, capacity int, fillRate float64) (bool, error) {
	if rl.rdb == nil {
		if rl.isProd {
			return false, fmt.Errorf("redis rate limiter unavailable in production mode")
		}
		return true, nil // Fallback open in non-prod if no redis
	}

	key := fmt.Sprintf("pcp:v1:ratelimit:%s:%s", scope, identifier)
	nowSec := time.Now().Unix()
	ttlSec := 3600 // 1 hour TTL

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

			switch scope {
			case ScopeLogin, ScopeWSUpgrade:
				identifier = ip
			case ScopeRestAPI:
				orgID := r.Header.Get("X-Organization-ID")
				if orgID == "" {
					orgID = "anon"
				}
				identifier = fmt.Sprintf("%s:%s", orgID, ip)
			case ScopeCommand:
				orgID := r.Header.Get("X-Organization-ID")
				userID := r.Header.Get("X-User-ID")
				deviceID := r.Header.Get("X-Device-ID")
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
