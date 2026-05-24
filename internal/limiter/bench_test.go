package limiter

import (
	"context"
	"testing"
)

type benchStubLimiter struct{}

func (benchStubLimiter) Decide(ctx context.Context, key string) (Decision, error) {
	_ = ctx
	_ = key
	return Decision{Allowed: true, Remaining: 42, Backend: "redis"}, nil
}

func BenchmarkTokenBucketAllowRequest(b *testing.B) {
	tb := NewTokenBucket(1000, 1000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tb.AllowRequest(1)
	}
}

func BenchmarkLocalLimiterDecide(b *testing.B) {
	l := NewLocalLimiter(1000, 1000)
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = l.Decide(ctx, "10.0.0.1")
	}
}

func BenchmarkOrchestratorRedisHit(b *testing.B) {
	o := NewOrchestrator(benchStubLimiter{}, NewLocalLimiter(1000, 1000))
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = o.Decide(ctx, "10.0.0.1")
	}
}
