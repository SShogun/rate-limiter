package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"rate-limiter/internal/limiter"
)

type benchMiddlewareLimiter struct{}

func (benchMiddlewareLimiter) Decide(ctx context.Context, key string) (limiter.Decision, error) {
	_ = ctx
	_ = key
	return limiter.Decision{Allowed: true, Remaining: 99, Backend: "redis"}, nil
}

func BenchmarkRateLimitMiddleware(b *testing.B) {
	mw := RateLimit(benchMiddlewareLimiter{})
	h := mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
	}
}
