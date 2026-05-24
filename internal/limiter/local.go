package limiter

import (
	"context"
	"math"
	"sync"
	"time"
)

type TokenBucket struct {
	mu                sync.Mutex
	maxBucketSize     float64
	refillRate        float64
	currentBucketSize float64
	lastRefillTime    int64
}

type LocalLimiter struct {
	mu            sync.RWMutex
	buckets       map[string]*TokenBucket
	maxBucketSize float64
	refillRate    float64
}

func NewTokenBucket(maxBucketSize float64, refillRate float64) *TokenBucket {
	return &TokenBucket{
		maxBucketSize:     maxBucketSize,
		refillRate:        refillRate,
		currentBucketSize: maxBucketSize,
		lastRefillTime:    time.Now().UnixNano(),
	}
}

func (tb *TokenBucket) refill() {
	now := time.Now().UnixNano()
	elapsed := now - tb.lastRefillTime

	// refillRate is tokens per second. elapsed is nanoseconds.
	tokensToAdd := (float64(elapsed) / 1e9) * tb.refillRate

	tb.currentBucketSize = math.Min(tb.maxBucketSize, tb.currentBucketSize+tokensToAdd)
	tb.lastRefillTime = now
}

func (tb *TokenBucket) AllowRequest(tokens int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	need := float64(tokens)
	if tb.currentBucketSize >= need {
		tb.currentBucketSize -= need
		return true
	}
	return false
}

func NewLocalLimiter(maxBucketSize float64, refillRate float64) *LocalLimiter {
	return &LocalLimiter{
		buckets:       make(map[string]*TokenBucket),
		maxBucketSize: maxBucketSize,
		refillRate:    refillRate,
	}
}

func (l *LocalLimiter) getBucket(key string) *TokenBucket {
	l.mu.RLock()
	b, ok := l.buckets[key]
	l.mu.RUnlock()
	if ok {
		return b
	}

	// create
	l.mu.Lock()
	defer l.mu.Unlock()
	// double-check
	if b, ok = l.buckets[key]; ok {
		return b
	}
	nb := NewTokenBucket(l.maxBucketSize, l.refillRate)
	l.buckets[key] = nb
	return nb
}

func (l *LocalLimiter) Decide(ctx context.Context, key string) (Decision, error) {
	_ = ctx

	b := l.getBucket(key)
	allowed := b.AllowRequest(1)
	return Decision{Allowed: allowed}, nil
}
