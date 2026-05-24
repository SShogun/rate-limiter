package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestNewRedisLimiterReturnsWorkingLimiter(t *testing.T) {
	limiterClient := newRedisLimiter("localhost:6379", 1, 1)
	if limiterClient == nil {
		t.Fatalf("expected limiter to be created")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	decision, err := limiterClient.Decide(ctx, fmt.Sprintf("cmd-api-test-%d", time.Now().UnixNano()))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			t.Skip("redis is not responding fast enough for the smoke test")
		}
		t.Fatalf("unexpected error from redis limiter: %v", err)
	}
	if decision.Backend != "redis" {
		t.Fatalf("expected redis backend, got %+v", decision)
	}
	if !decision.Allowed {
		t.Fatalf("expected first request to be allowed, got %+v", decision)
	}
}
