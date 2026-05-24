package limiter

import (
	"context"
	"errors"
	"testing"
	"time"
)

type stubLimiter struct {
	decision Decision
	err      error
	calls    int
}

func (s *stubLimiter) Decide(ctx context.Context, key string) (Decision, error) {
	_ = ctx
	_ = key
	s.calls++
	return s.decision, s.err
}

func TestOrchestratorUsesRedisWhenAvailable(t *testing.T) {
	redisLimiter := &stubLimiter{decision: Decision{Allowed: true, Remaining: 7, Backend: "redis"}}
	localLimiter := &stubLimiter{decision: Decision{Allowed: true, Remaining: 1, Backend: "local"}}
	o := NewOrchestrator(redisLimiter, localLimiter)

	decision, err := o.Decide(context.Background(), "10.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed || decision.Backend != "redis" {
		t.Fatalf("expected redis decision, got %+v", decision)
	}
	if redisLimiter.calls != 1 {
		t.Fatalf("expected redis limiter to be called once, got %d", redisLimiter.calls)
	}
	if localLimiter.calls != 0 {
		t.Fatalf("expected local limiter not to be called, got %d", localLimiter.calls)
	}
}

func TestOrchestratorFallsBackToLocalOnRedisFailure(t *testing.T) {
	redisLimiter := &stubLimiter{err: errors.New("redis down")}
	localLimiter := &stubLimiter{decision: Decision{Allowed: true, Remaining: 3, Backend: "local"}}
	o := NewOrchestrator(redisLimiter, localLimiter)

	decision, err := o.Decide(context.Background(), "10.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allowed || decision.Backend != "local" {
		t.Fatalf("expected local fallback, got %+v", decision)
	}
	if redisLimiter.calls != 1 {
		t.Fatalf("expected redis limiter to be called once, got %d", redisLimiter.calls)
	}
	if localLimiter.calls != 1 {
		t.Fatalf("expected local limiter to be called once, got %d", localLimiter.calls)
	}
}

func TestOrchestratorOpensBreakerAfterFailures(t *testing.T) {
	redisLimiter := &stubLimiter{err: errors.New("redis down")}
	localLimiter := &stubLimiter{decision: Decision{Allowed: true, Remaining: 4, Backend: "local"}}
	o := NewOrchestrator(redisLimiter, localLimiter)
	o.threshold = 1

	_, err := o.Decide(context.Background(), "10.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if o.state != "open" {
		t.Fatalf("expected breaker to open, got %q", o.state)
	}

	_, err = o.Decide(context.Background(), "10.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error on open breaker path: %v", err)
	}
	if redisLimiter.calls != 1 {
		t.Fatalf("expected redis limiter not to be retried while breaker is open, got %d calls", redisLimiter.calls)
	}
	if localLimiter.calls != 2 {
		t.Fatalf("expected local limiter to serve both requests, got %d", localLimiter.calls)
	}
}

func TestOrchestratorHalfOpenRetriesRedis(t *testing.T) {
	redisLimiter := &stubLimiter{err: errors.New("redis down")}
	localLimiter := &stubLimiter{decision: Decision{Allowed: true, Remaining: 2, Backend: "local"}}
	o := NewOrchestrator(redisLimiter, localLimiter)
	o.state = "open"
	o.lastFailureTime = time.Now().Add(-6 * time.Second)

	decision, err := o.Decide(context.Background(), "10.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Backend != "local" {
		t.Fatalf("expected local fallback in half-open failure path, got %+v", decision)
	}
	if o.state != "open" {
		t.Fatalf("expected breaker to remain open after half-open failure, got %q", o.state)
	}
	if redisLimiter.calls != 1 {
		t.Fatalf("expected redis limiter to be attempted once, got %d", redisLimiter.calls)
	}
}