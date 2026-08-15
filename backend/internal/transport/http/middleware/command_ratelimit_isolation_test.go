package middleware_test

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	custommw "github.com/tcandt/cloudphone-farm/backend/internal/transport/http/middleware"
)

func TestCommandRateLimitPerDeviceIsolation(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	limiter := custommw.NewRateLimiter(rdb, "development")
	ctx := context.Background()

	orgID := "org_acme"
	userID := "user_operator"
	deviceA := "dev_samsung_a"
	deviceB := "dev_samsung_b"

	keyA := orgID + ":" + userID + ":" + deviceA
	keyB := orgID + ":" + userID + ":" + deviceB

	capacity := 2
	fillRate := 0.1

	// Exhaust device A bucket (2 allowed)
	ok1, err := limiter.CheckLimit(ctx, custommw.ScopeCommand, keyA, capacity, fillRate)
	if !ok1 || err != nil {
		t.Fatalf("expected request 1 for device A to pass, got ok=%v, err=%v", ok1, err)
	}

	ok2, err := limiter.CheckLimit(ctx, custommw.ScopeCommand, keyA, capacity, fillRate)
	if !ok2 || err != nil {
		t.Fatalf("expected request 2 for device A to pass, got ok=%v, err=%v", ok2, err)
	}

	// 3rd request for device A MUST BE REJECTED
	ok3, err := limiter.CheckLimit(ctx, custommw.ScopeCommand, keyA, capacity, fillRate)
	if ok3 {
		t.Fatalf("expected 3rd request for device A to be rejected by rate limiter")
	}

	// Device B bucket MUST REMAIN FRESH & UNTOUCHED (must pass)
	okB1, err := limiter.CheckLimit(ctx, custommw.ScopeCommand, keyB, capacity, fillRate)
	if !okB1 || err != nil {
		t.Fatalf("expected request 1 for device B to pass despite device A exhaustion, got ok=%v, err=%v", okB1, err)
	}
}
