package main

import (
	"errors"
	"net/http"
	"rate-limiter/internal/limiter"
	"rate-limiter/internal/middleware"
	"time"

	"github.com/go-chi/chi/v5"
)

func main() {
	r := chi.NewRouter()
	redisLimiter := limiter.NewRedisLimiter("localhost:6379", 10, 2)
	localLimiter := limiter.NewLocalLimiter(10, 2)
	orchestrator := limiter.NewOrchestrator(redisLimiter, localLimiter)

	r.Use(middleware.RateLimit(orchestrator))
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("hello"))
	})

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}
