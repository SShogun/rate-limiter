package limiter

import (
	"context"
	"sync"
	"time"
)

type BreakerState string

const (
	BreakerClosed   BreakerState = "closed"
	BreakerOpen     BreakerState = "open"
	BreakerHalfOpen BreakerState = "half-open"
)

type Orchestrator struct {
	redis Limiter
	local Limiter

	failures  int
	threshold int

	state BreakerState

	lastFailureTime time.Time

	mu sync.Mutex
}

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

func (o *Orchestrator) Decide(ctx context.Context, key string) (Decision, error) {

	const breakerTimeout = 5 * time.Second

	// ----- Read/update breaker state -----

	o.mu.Lock()

	currentState := o.state

	if currentState == BreakerOpen {

		if time.Since(
			o.lastFailureTime,
		) < breakerTimeout {

			o.mu.Unlock()

			return o.local.Decide(
				ctx,
				key,
			)
		}

		o.state = BreakerHalfOpen
		currentState = BreakerHalfOpen
	}

	o.mu.Unlock()

	// ----- Try Redis outside lock -----

	redisDecision, err := o.redis.Decide(
		ctx,
		key,
	)

	if err == nil {

		o.mu.Lock()

		o.failures = 0
		o.state = BreakerClosed

		o.mu.Unlock()

		return redisDecision, nil
	}

	// ----- Redis failed in half-open mode -----

	if currentState == BreakerHalfOpen {

		o.mu.Lock()

		o.state = BreakerOpen
		o.lastFailureTime = time.Now()

		o.mu.Unlock()

		return o.local.Decide(
			ctx,
			key,
		)
	}

	// ----- Normal Redis failure path -----

	o.mu.Lock()

	o.failures++
	o.lastFailureTime = time.Now()

	if o.failures >= o.threshold {
		o.state = BreakerOpen
	}

	o.mu.Unlock()

	return o.local.Decide(
		ctx,
		key,
	)
}
