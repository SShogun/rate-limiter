// Package limiter holds the hybrid Redis + in-memory limiter primitives.
// The package stays deliberately small; the interesting bit is how the pieces
// are wired together rather than any one algorithm in isolation.
package limiter

import (
	"context"
	"time"
)

// Decision is the answer every limiter returns. The middleware turns this into
// HTTP headers, so the shape is intentionally boring and explicit.
type Decision struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
	Backend    string
}

// Limiter is the tiny contract shared by Redis, the local fallback, and the
// circuit breaker orchestrator.
type Limiter interface {
	Decide(ctx context.Context, key string) (Decision, error)
}
