package limiter

import (
	"context"
	"sync"
	"time"
)

// BreakerState names the circuit-breaker state.
type BreakerState string

const (
	// BreakerClosed means Redis is healthy enough to be the primary path.
	BreakerClosed BreakerState = "closed"
	// BreakerOpen means Redis is skipped and the local limiter handles traffic.
	BreakerOpen BreakerState = "open"
	// BreakerHalfOpen is the probe state after the cooldown window.
	BreakerHalfOpen BreakerState = "half-open"
)

// Orchestrator is the circuit-breaker wrapper. Redis is the happy path; the
// in-memory limiter is there so we still behave sanely when the network is not.
type Orchestrator struct {
	redis Limiter
	local Limiter

	failures  int
	threshold int

	state BreakerState

	lastFailureTime time.Time

	mu sync.Mutex
}

// NewOrchestrator wires Redis and local fallback together with a small breaker
// that opens after a few failures.
func NewOrchestrator(
	redis Limiter,
	local Limiter,
) *Orchestrator {

	return &Orchestrator{
		redis:     redis,
		local:     local,
		threshold: 3,
		state:     BreakerClosed,
	}
}

// Decide prefers Redis, falls back to local on failure, and rechecks Redis once
// the breaker has cooled down.
func (o *Orchestrator) Decide(ctx context.Context, key string) (Decision, error) {
	const breakerTimeout = 5 * time.Second

	o.mu.Lock()
	currentState := o.state
	if currentState == BreakerOpen {
		if time.Since(o.lastFailureTime) < breakerTimeout {
			o.mu.Unlock()
			return o.local.Decide(ctx, key)
		}
		o.state = BreakerHalfOpen
		currentState = BreakerHalfOpen
	}
	o.mu.Unlock()

	redisDecision, err := o.redis.Decide(ctx, key)
	if err == nil {
		o.mu.Lock()
		o.failures = 0
		o.state = BreakerClosed
		o.mu.Unlock()
		return redisDecision, nil
	}

	if currentState == BreakerHalfOpen {
		o.mu.Lock()
		o.state = BreakerOpen
		o.lastFailureTime = time.Now()
		o.mu.Unlock()
		return o.local.Decide(ctx, key)
	}
	o.mu.Lock()
	o.failures++
	o.lastFailureTime = time.Now()
	if o.failures >= o.threshold {
		o.state = BreakerOpen
	}
	o.mu.Unlock()
	return o.local.Decide(ctx, key)
}
