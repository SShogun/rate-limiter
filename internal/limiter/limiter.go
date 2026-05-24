package limiter

import (
	"context"
	"time"
)

type Decision struct {
	Allowed    bool
	Remaining  int
	RetryAfter time.Duration
	Backend    string
}

type Limiter interface {
	Decide(ctx context.Context, key string) (Decision, error)
}
