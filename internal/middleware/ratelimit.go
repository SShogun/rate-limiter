package middleware

import (
	"net"
	"net/http"
	"strconv"

	"rate-limiter/internal/limiter"
)

func RateLimit(l limiter.Limiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				ip, _, err := net.SplitHostPort(
					r.RemoteAddr,
				)
				if err != nil {
					http.Error(
						w,
						"invalid client",
						http.StatusBadRequest,
					)
					return
				}
				decision, err := l.Decide(
					r.Context(),
					ip,
				)
				if err != nil {
					http.Error(
						w,
						"internal server error",
						http.StatusInternalServerError,
					)
					return
				}
				w.Header().Set(
					"X-RateLimit-Remaining",
					strconv.Itoa(
						decision.Remaining,
					),
				)
				w.Header().Set(
					"Retry-After",
					strconv.Itoa(
						int(
							decision.RetryAfter.Seconds(),
						),
					),
				)
				w.Header().Set(
					"X-RateLimit-Backend",
					decision.Backend,
				)
				if !decision.Allowed {
					http.Error(
						w,
						"rate limit exceeded",
						http.StatusTooManyRequests,
					)
					return
				}
				next.ServeHTTP(
					w,
					r,
				)
			},
		)
	}
}
