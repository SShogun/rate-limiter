// Package examples contains the small scratch code that informed the production
// limiter. It is intentionally plain and self-contained.
package examples

import (
	"math"
	"sync"
	"time"
)

// TokenBucket is the example version of the in-memory bucket. It exists as a
// learning aid rather than a production dependency.
type TokenBucket struct {
	mu                sync.Mutex
	maxBucketSize     float64
	refillRate        float64
	currentBucketSize float64
	lastRefillAt      int64
}

// NewTokenBucket creates a full bucket with the given capacity and refill
// rate.
func NewTokenBucket(maxBucketSize float64, refillRate float64) *TokenBucket {
	return &TokenBucket{
		maxBucketSize:     maxBucketSize,
		refillRate:        refillRate,
		currentBucketSize: maxBucketSize,
		lastRefillAt:      time.Now().UnixNano(),
	}
}

// AllowRequest spends tokens when the bucket can afford it.
func (tb *TokenBucket) AllowRequest(tokens int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()

	if tb.currentBucketSize >= float64(tokens) {
		tb.currentBucketSize -= float64(tokens)
		return true
	}
	return false
}

func (tb *TokenBucket) refill() {
	now := time.Now().UnixNano()

	tokensToAdd := float64(now-tb.lastRefillAt) * tb.refillRate / 1e9

	tb.currentBucketSize = math.Min(tb.currentBucketSize+tokensToAdd, tb.maxBucketSize)
	tb.lastRefillAt = now
}
