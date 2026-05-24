package limiter

import (
	"context"
	"testing"
	"time"
)

func TestTokenBucketAllowRequest(t *testing.T) {
	tb := NewTokenBucket(2, 0)
	if !tb.AllowRequest(1) {
		t.Fatalf("expected first request to be allowed")
	}
	if !tb.AllowRequest(1) {
		t.Fatalf("expected second request to be allowed")
	}
	if tb.AllowRequest(1) {
		t.Fatalf("expected third request to be denied")
	}
}

func TestLocalLimiterSeparatesKeys(t *testing.T) {
	l := NewLocalLimiter(1, 0)
	if decision, err := l.Decide(context.Background(), "10.0.0.1"); err != nil || !decision.Allowed {
		t.Fatalf("expected first key request to be allowed, got decision=%+v err=%v", decision, err)
	}
	if decision, err := l.Decide(context.Background(), "10.0.0.1"); err != nil || decision.Allowed {
		t.Fatalf("expected second request for same key to be denied, got decision=%+v err=%v", decision, err)
	}
	if decision, err := l.Decide(context.Background(), "10.0.0.2"); err != nil || !decision.Allowed {
		t.Fatalf("expected different key to have its own bucket, got decision=%+v err=%v", decision, err)
	}
}

func TestTokenBucketRefill(t *testing.T) {
	tb := NewTokenBucket(1, 10)
	if !tb.AllowRequest(1) {
		t.Fatalf("expected request to be allowed")
	}
	time.Sleep(120 * time.Millisecond)
	if !tb.AllowRequest(1) {
		t.Fatalf("expected token bucket to refill")
	}
}
