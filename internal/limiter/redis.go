package limiter

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisLimiter struct {
	client        *redis.Client
	maxBucketSize float64
	refillRate    float64
}

// NewRedisLimiter creates a Redis-backed token-bucket limiter. `max` is the
// bucket capacity (tokens) and `refillRate` is tokens per second.
func NewRedisLimiter(addr string, max int, refillRate int) *RedisLimiter {
	c := redis.NewClient(&redis.Options{Addr: addr})
	return &RedisLimiter{
		client:        c,
		maxBucketSize: float64(max),
		refillRate:    float64(refillRate),
	}
}

var tokenBucketLua = redis.NewScript(`
local key = KEYS[1]

local capacity = tonumber(ARGV[1])
local refillRate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])

local tokens = tonumber(redis.call("HGET", key, "tokens"))
local lastRefill = tonumber(redis.call("HGET", key, "last_refill"))

if not tokens then
	tokens = capacity
	lastRefill = now
end

local elapsed = now - lastRefill
local tokensToAdd = elapsed * refillRate

if tokens + tokensToAdd > capacity then
	tokens = capacity
else
	tokens = tokens + tokensToAdd
end

local allowed = 0
local retryAfter = 0

if tokens >= 1 then
	allowed = 1
	tokens = tokens - 1
else
	retryAfter = math.ceil(1 / refillRate)
end

redis.call("HSET", key, "tokens", tokens, "last_refill", now)

return {allowed, math.floor(tokens), retryAfter}
`)

func (r *RedisLimiter) Decide(ctx context.Context, key string) (Decision, error) {
	redisKey := "redis:" + key

	// use seconds as float for the script
	now := float64(time.Now().UnixNano()) / 1e9

	res, err := tokenBucketLua.Run(ctx, r.client, []string{redisKey}, r.maxBucketSize, r.refillRate, now).Result()
	if err != nil {
		return Decision{}, err
	}

	vals, ok := res.([]interface{})
	if !ok || len(vals) < 3 {
		return Decision{}, nil
	}

	toInt64 := func(v interface{}) int64 {
		switch x := v.(type) {
		case int64:
			return x
		case int:
			return int64(x)
		case float64:
			return int64(x)
		default:
			return 0
		}
	}

	allowed := toInt64(vals[0])
	remaining := toInt64(vals[1])
	retryAfter := toInt64(vals[2])

	return Decision{
		Allowed:    allowed == 1,
		Remaining:  int(remaining),
		RetryAfter: time.Duration(retryAfter) * time.Second,
		Backend:    "redis",
	}, nil
}
