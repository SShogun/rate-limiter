// Package middleware exposes the HTTP glue around the limiter package. This is
// intentionally small; the interesting bits live in the limiter itself.
package middleware

import (
	"net"
	"net/http"
	"strconv"

	"rate-limiter/internal/limiter"
)

// RateLimit wraps an HTTP handler with the limiter contract and emits the
// headers the rest of the project uses for debugging.
func RateLimit(l limiter.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				http.Error(w, "invalid client", http.StatusBadRequest)
				return
			}

			dec, err := l.Decide(r.Context(), ip)
			if err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			h := w.Header()
			h.Set("X-RateLimit-Remaining", strconv.Itoa(dec.Remaining))
			h.Set("Retry-After", strconv.Itoa(int(dec.RetryAfter.Seconds())))
			h.Set("X-RateLimit-Backend", dec.Backend)
			if !dec.Allowed {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
