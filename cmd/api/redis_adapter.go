package main

import "rate-limiter/internal/limiter"

// newRedisLimiter is a small adapter so the existing main.go can call the
// constructor without changing imports. It forwards parameters to the
// limiter package's NewRedisLimiter factory.
func newRedisLimiter(addr string, max int, windowSeconds int) limiter.Limiter {
	return limiter.NewRedisLimiter(addr, max, windowSeconds)
}
