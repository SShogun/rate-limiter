# rate-limiter

[![Go Version](https://img.shields.io/badge/go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen.svg)](.)

A small Go rate-limiter built to stay useful when Redis is healthy and still behave sensibly when Redis is not. The core idea is a hybrid design: Redis is the primary store, an in-memory token bucket acts as the fallback, and a circuit breaker decides when to stop paying the network tax.

This is not a framework demo. It is a practical learning project that tries to answer the annoying questions that show up in real systems: what happens when Redis blips, where does the fallback live, how do headers get set, and what should the middleware tell callers when they are throttled?

## Features

- Redis-backed token bucket for the fast path.
- In-memory token bucket fallback for resilience.
- Circuit breaker orchestration between the two.
- HTTP middleware that exposes standard rate-limit headers.
- Simple example server under `cmd/api`.
- Tests for limiter behavior, middleware behavior, and the breaker.
- Benchmarks for the hot paths.

## Why this exists

I wanted a rate limiter that was small enough to read in one sitting, but still had the moving parts you would expect in a real service: shared state, fallback behavior, and a bit of failure handling. A lot of sample code stops at `go run`; this one tries to be useful when the network is flaky.

## How it works

The request flow is intentionally straightforward:

1. Middleware extracts the client IP.
2. The orchestrator asks Redis first.
3. If Redis succeeds, that decision wins.
4. If Redis fails, the breaker counts it and the local limiter takes over.
5. Once the breaker trips, Redis is skipped for a short cooldown window.
6. After the cooldown, Redis gets another chance in half-open mode.

The local limiter is an in-memory token bucket keyed by client IP. It is cheap, fast, and disposable. That is the right trade-off for fallback logic.

## Project structure

```text
cmd/api              Example HTTP server
examples             Scratch/example code used while building the limiter
internal/limiter     Token bucket, Redis limiter, and circuit breaker orchestration
internal/middleware   HTTP middleware that turns limiter decisions into headers
```

## Installation

```bash
git clone https://github.com/SShogun/rate-limiter.git
cd rate-limiter
go mod download
```

If you want to run the Redis-backed path locally, start Redis first:

```bash
docker run -p 6379:6379 redis
```

## Quick start

Run the example server:

```bash
go run ./cmd/api
```

Then hit it a few times:

```bash
curl -i http://localhost:8080/
```

When the limit is hit, the middleware returns `429 Too Many Requests` and includes headers such as:

- `X-RateLimit-Remaining`
- `Retry-After`
- `X-RateLimit-Backend`

## Usage

### Middleware

```go
router := chi.NewRouter()
redisLimiter := limiter.NewRedisLimiter("localhost:6379", 10, 2)
localLimiter := limiter.NewLocalLimiter(10, 2)
orchestrator := limiter.NewOrchestrator(redisLimiter, localLimiter)

router.Use(middleware.RateLimit(orchestrator))
```

### Direct limiter use

```go
dec, err := orchestrator.Decide(context.Background(), "192.168.1.10")
if err != nil {
    return err
}

if !dec.Allowed {
    fmt.Printf("try again in %s\n", dec.RetryAfter)
}
```

## Configuration

The example server is intentionally minimal and currently hard-codes the common demo values:

- Redis address: `localhost:6379`
- Capacity: `10`
- Refill rate: `2` tokens per second
- HTTP listen address: `:8080`

If you turn this into a real service, I would make those values environment-driven first. That is the simplest production improvement with the highest payoff.

## Benchmarks

The repository includes benchmarks for the main hot paths. On my local Windows machine (13th Gen Intel Core i5-13450HX, Go 1.25), the measured results were:

```text
BenchmarkTokenBucketAllowRequest    47.21 ns/op    0 B/op   0 allocs/op
BenchmarkLocalLimiterDecide         159.6 ns/op    0 B/op   0 allocs/op
BenchmarkOrchestratorRedisHit       85.40 ns/op    0 B/op   0 allocs/op
BenchmarkRateLimitMiddleware        1124 ns/op     608 B/op 9 allocs/op
```

The middleware benchmark is naturally a little noisier because it exercises HTTP plumbing and header writes. Redis-backed numbers will move around more in a real deployment, so treat the values above as a local snapshot rather than a promise.

Run them with:

```bash
make bench
```

## Circuit breaker behavior

The breaker is intentionally plain:

- `closed`: Redis is queried normally.
- `open`: Redis is skipped and the local limiter handles requests.
- `half-open`: Redis gets a probe request after the cooldown window.

The implementation is conservative on purpose. It favors serving requests through the local limiter rather than stalling or failing closed whenever Redis is twitchy.

## Limitations

- The local fallback is process-local, so it does not coordinate across multiple app instances.
- Client identity is derived from `RemoteAddr`, which is fine for a demo but not enough behind a proxy without real `X-Forwarded-For` handling.
- The Redis limiter is intentionally small and does not try to solve every distributed-systems edge case.
- Bucket cleanup is lazy and best-effort. That is good enough here, but I would revisit it before putting this in a high-churn multi-tenant service.

## Production recommendations

- Make Redis address, capacity, refill rate, and listen address configurable.
- Add graceful shutdown to `cmd/api`.
- Use trusted proxy parsing if the service sits behind a load balancer.
- Export metrics for limiter decisions, breaker state, and Redis failures.
- Put the Redis path behind an integration test in CI.
- Consider per-route or per-user policies instead of a single global bucket shape.

## Testing

```bash
go test ./...
```

For the race detector:

```bash
go test -race ./...
```

## License

MIT. See [LICENSE](LICENSE).