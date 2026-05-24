package limiter

import (
	"context"
	"math"
	"sync"
	"time"
)

const (
	defaultBucketTTL       = 5 * time.Minute
	defaultCleanupInterval = 1 * time.Minute
)

type TokenBucket struct {
	mu                sync.Mutex
	maxBucketSize     float64
	refillRate        float64
	currentBucketSize float64
	lastRefillTime    int64
	lastAccess        time.Time
}

type LocalLimiter struct {
	mu            sync.RWMutex
	buckets       map[string]*TokenBucket
	maxBucketSize float64
	refillRate    float64
	bucketTTL     time.Duration
}

func NewTokenBucket(maxBucketSize float64, refillRate float64) *TokenBucket {
	return &TokenBucket{
		maxBucketSize:     maxBucketSize,
		refillRate:        refillRate,
		currentBucketSize: maxBucketSize,
		lastRefillTime:    time.Now().UnixNano(),
		lastAccess:        time.Now(),
	}
}

func (tb *TokenBucket) touch() {
	tb.mu.Lock()
	tb.lastAccess = time.Now()
	tb.mu.Unlock()
}

func (tb *TokenBucket) refill() {
	now := time.Now().UnixNano()
	elapsed := now - tb.lastRefillTime

	// refillRate is tokens per second. elapsed is nanoseconds.
	tokensToAdd := (float64(elapsed) / 1e9) * tb.refillRate

	tb.currentBucketSize = math.Min(tb.maxBucketSize, tb.currentBucketSize+tokensToAdd)
	tb.lastRefillTime = now
}

func (tb *TokenBucket) RemainingTokens() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	return int(math.Floor(tb.currentBucketSize))
}

func (tb *TokenBucket) RetryAfter() time.Duration {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	if tb.currentBucketSize >= 1 || tb.refillRate <= 0 {
		return 0
	}

	seconds := (1 - tb.currentBucketSize) / tb.refillRate
	if seconds < 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

func (tb *TokenBucket) AllowRequest(tokens int) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	tb.lastAccess = time.Now()
	need := float64(tokens)
	if tb.currentBucketSize >= need {
		tb.currentBucketSize -= need
		return true
	}
	return false
}

func NewLocalLimiter(maxBucketSize float64, refillRate float64) *LocalLimiter {
	l := &LocalLimiter{
		buckets:       make(map[string]*TokenBucket),
		maxBucketSize: maxBucketSize,
		refillRate:    refillRate,
		bucketTTL:     defaultBucketTTL,
	}
	go l.cleanupLoop(defaultCleanupInterval)
	return l
}

func (l *LocalLimiter) getBucket(key string) *TokenBucket {
	l.mu.RLock()
	b, ok := l.buckets[key]
	l.mu.RUnlock()
	if ok {
		b.touch()
		return b
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if b, ok = l.buckets[key]; ok {
		b.touch()
		return b
	}

	nb := NewTokenBucket(l.maxBucketSize, l.refillRate)
	nb.touch()
	l.buckets[key] = nb
	return nb
}

func (l *LocalLimiter) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		l.cleanupExpiredBuckets(time.Now())
	}
}

func (l *LocalLimiter) cleanupExpiredBuckets(now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for key, bucket := range l.buckets {
		bucket.mu.Lock()
		stale := now.Sub(bucket.lastAccess) > l.bucketTTL
		bucket.mu.Unlock()

		if stale {
			delete(l.buckets, key)
		}
	}
}

func (l *LocalLimiter) Decide(ctx context.Context, key string) (Decision, error) {
	select {
	case <-ctx.Done():
		return Decision{}, ctx.Err()
	default:
	}

	b := l.getBucket(key)
	allowed := b.AllowRequest(1)
	decision := Decision{
		Allowed:   allowed,
		Remaining: b.RemainingTokens(),
		Backend:   "local",
	}
	if !allowed {
		decision.RetryAfter = b.RetryAfter()
	}
	return decision, nil
}
