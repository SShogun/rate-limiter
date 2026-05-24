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
	if decision, err := l.Decide(context.Background(), "10.0.0.1"); err != nil || !decision.Allowed || decision.Backend != "local" || decision.Remaining != 0 {
		t.Fatalf("expected first key request to be allowed, got decision=%+v err=%v", decision, err)
	}
	if decision, err := l.Decide(context.Background(), "10.0.0.1"); err != nil || decision.Allowed || decision.Backend != "local" {
		t.Fatalf("expected second request for same key to be denied, got decision=%+v err=%v", decision, err)
	}
	if decision, err := l.Decide(context.Background(), "10.0.0.2"); err != nil || !decision.Allowed || decision.Backend != "local" {
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

func TestLocalLimiterCleansUpStaleBuckets(t *testing.T) {
	l := NewLocalLimiter(1, 0)
	_ = l.getBucket("10.0.0.1")

	l.bucketTTL = time.Millisecond
	l.mu.Lock()
	bucket := l.buckets["10.0.0.1"]
	l.mu.Unlock()

	bucket.mu.Lock()
	bucket.lastAccess = time.Now().Add(-time.Second)
	bucket.mu.Unlock()

	l.cleanupExpiredBuckets(time.Now())

	l.mu.RLock()
	_, ok := l.buckets["10.0.0.1"]
	l.mu.RUnlock()
	if ok {
		t.Fatalf("expected stale bucket to be removed")
	}
}
