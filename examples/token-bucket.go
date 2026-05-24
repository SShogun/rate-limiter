package examples

import (
	"math"
	"sync"
	"time"
)

type TokenBucket struct {
	mu                  sync.Mutex
	maxBucketSize       float64
	refillRate          float64
	currentBucketSize   float64
	lastRefillTimeStamp int64
}

func NewTokenBucket(maxBucketSize float64, refillRate float64) *TokenBucket {
	return &TokenBucket{
		maxBucketSize:       maxBucketSize,
		refillRate:          refillRate,
		currentBucketSize:   maxBucketSize,
		lastRefillTimeStamp: time.Now().UnixNano(),
	}
}

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

	tokensToAdd := float64(now-tb.lastRefillTimeStamp) * tb.refillRate / 1e9

	tb.currentBucketSize = math.Min(tb.currentBucketSize+tokensToAdd, tb.maxBucketSize)
	tb.lastRefillTimeStamp = now
}
