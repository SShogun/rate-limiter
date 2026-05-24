package limiter

import (
	"context"
	"sync"
	"time"
)

type Orchestrator struct {
	redis Limiter
	local Limiter

	failures  int
	threshold int

	state string

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
		state:     "closed",
	}
}

func (o *Orchestrator) Decide(ctx context.Context, key string) (Decision, error) {

	const breakerTimeout = 5 * time.Second

	// ----- Read/update breaker state -----

	o.mu.Lock()

	currentState := o.state

	if currentState == "open" {

		if time.Since(
			o.lastFailureTime,
		) < breakerTimeout {

			o.mu.Unlock()

			return o.local.Decide(
				ctx,
				key,
			)
		}

		o.state = "half-open"
		currentState = "half-open"
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
		o.state = "closed"

		o.mu.Unlock()

		return redisDecision, nil
	}

	// ----- Redis failed in half-open mode -----

	if currentState == "half-open" {

		o.mu.Lock()

		o.state = "open"
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
		o.state = "open"
	}

	o.mu.Unlock()

	return o.local.Decide(
		ctx,
		key,
	)
}
