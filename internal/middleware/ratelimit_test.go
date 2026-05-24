package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rate-limiter/internal/limiter"
)

type middlewareStubLimiter struct {
	decision limiter.Decision
	err      error
	calls    int
}

func (s *middlewareStubLimiter) Decide(ctx context.Context, key string) (limiter.Decision, error) {
	_ = ctx
	_ = key
	s.calls++
	return s.decision, s.err
}

func TestRateLimitAllowsRequest(t *testing.T) {
	l := &middlewareStubLimiter{decision: limiter.Decision{Allowed: true, Remaining: 4, RetryAfter: 0, Backend: "redis"}}
	mw := RateLimit(l)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	called := false

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)

	if !called {
		t.Fatalf("expected next handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 response, got %d", rec.Code)
	}
	if got := rec.Header().Get("X-RateLimit-Remaining"); got != "4" {
		t.Fatalf("expected remaining header 4, got %q", got)
	}
	if got := rec.Header().Get("X-RateLimit-Backend"); got != "redis" {
		t.Fatalf("expected backend header redis, got %q", got)
	}
	if l.calls != 1 {
		t.Fatalf("expected limiter to be called once, got %d", l.calls)
	}
}

func TestRateLimitRejectsWhenDenied(t *testing.T) {
	l := &middlewareStubLimiter{decision: limiter.Decision{Allowed: false, Remaining: 0, RetryAfter: 2 * time.Second, Backend: "local"}}
	mw := RateLimit(l)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called when rate limited")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 response, got %d", rec.Code)
	}
	if got := rec.Header().Get("Retry-After"); got != "2" {
		t.Fatalf("expected retry-after header 2, got %q", got)
	}
	if got := rec.Header().Get("X-RateLimit-Backend"); got != "local" {
		t.Fatalf("expected backend header local, got %q", got)
	}
}

func TestRateLimitRejectsInvalidRemoteAddr(t *testing.T) {
	l := &middlewareStubLimiter{decision: limiter.Decision{Allowed: true, Backend: "redis"}}
	mw := RateLimit(l)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "invalid"
	rec := httptest.NewRecorder()

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 response, got %d", rec.Code)
	}
}

func TestRateLimitReturnsInternalErrorOnLimiterFailure(t *testing.T) {
	l := &middlewareStubLimiter{err: errors.New("boom")}
	mw := RateLimit(l)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()

	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 response, got %d", rec.Code)
	}
}