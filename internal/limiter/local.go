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

// TokenBucket is the in-memory bucket used for the fallback path. It is kept
// intentionally simple because this is the code that gets exercised when Redis
// is misbehaving.
type TokenBucket struct {
	mu                sync.Mutex
	maxBucketSize     float64
	refillRate        float64
	currentBucketSize float64
	lastRefillTime    int64
	lastAccess        time.Time
}

// LocalLimiter keeps a bucket per key in memory. It trades memory for speed,
// which is the right call for a fallback path that should never block on IO.
type LocalLimiter struct {
	mu            sync.RWMutex
	buckets       map[string]*TokenBucket
	maxBucketSize float64
	refillRate    float64
	bucketTTL     time.Duration
}

// NewTokenBucket creates a bucket already full. That's the common case and it
// avoids a separate warm-up step.
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

func (tb *TokenBucket) remainingAndRetryLocked() (int, time.Duration) {
	remaining := int(math.Floor(tb.currentBucketSize))
	if remaining >= 1 || tb.refillRate <= 0 {
		return remaining, 0
	}

	seconds := (1 - tb.currentBucketSize) / tb.refillRate
	if seconds <= 0 {
		return remaining, 0
	}
	return remaining, time.Duration(seconds * float64(time.Second))
}

// RemainingTokens reports the current number of tokens after applying a refill
// based on the wall clock.
func (tb *TokenBucket) RemainingTokens() int {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	remaining, _ := tb.remainingAndRetryLocked()
	return remaining
}

// RetryAfter estimates how long the caller should wait before retrying.
func (tb *TokenBucket) RetryAfter() time.Duration {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	tb.refill()
	_, retry := tb.remainingAndRetryLocked()
	return retry
}

// AllowRequest spends tokens if they are available.
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

// NewLocalLimiter returns the fallback limiter used when Redis is down or the
// breaker is open.
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
	// This is a bit ugly, but it avoids dragging cleanup into the hot path.
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

// Decide applies the token bucket and returns a fully populated Decision. The
// middleware depends on the headers, so we keep the response shape consistent
// even in fallback mode.
func (l *LocalLimiter) Decide(ctx context.Context, key string) (Decision, error) {
	select {
	case <-ctx.Done():
		return Decision{}, ctx.Err()
	default:
	}

	b := l.getBucket(key)
	allowed := b.AllowRequest(1)
	b.mu.Lock()
	remaining, retry := b.remainingAndRetryLocked()
	b.mu.Unlock()

	decision := Decision{
		Allowed:   allowed,
		Remaining: remaining,
		Backend:   "local",
	}
	if !allowed {
		decision.RetryAfter = retry
	}
	return decision, nil
}
