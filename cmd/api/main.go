package main

import (
	"net/http"
	"rate-limiter/internal/limiter"
	"rate-limiter/internal/middleware"

	"github.com/go-chi/chi/v5"
)

func main() {

	// l := limiter.NewLocalLimiter(
	// 	10,
	// 	2,
	// )

	// for i := 0; i < 15; i++ {

	// 	d, err := l.Decide(
	// 		context.Background(),
	// 		"192.168.1.1",
	// 	)

	// 	if err != nil {
	// 		panic(err)
	// 	}

	// 	fmt.Println(
	// 		d.Allowed,
	// 		d.Remaining,
	// 	)
	// }

	// r := newRedisLimiter(
	// 	"localhost:6379",
	// 	5,
	// 	1,
	// )

	// for i := 0; i < 10; i++ {

	// 	d, err := r.Decide(
	// 		context.Background(),
	// 		"192.168.1.1",
	// 	)

	// 	if err != nil {
	// 		panic(err)
	// 	}

	// 	fmt.Println(
	// 		d.Allowed,
	// 		d.Remaining,
	// 		d.RetryAfter,
	// 	)
	// }

	r := chi.NewRouter()

	redisLimiter := limiter.NewRedisLimiter(
		"localhost:6379",
		10,
		2,
	)

	localLimiter := limiter.NewLocalLimiter(
		10,
		2,
	)

	orchestrator := limiter.NewOrchestrator(
		redisLimiter,
		localLimiter,
	)

	r.Use(
		middleware.RateLimit(
			orchestrator,
		),
	)

	r.Get(
		"/",
		func(
			w http.ResponseWriter,
			r *http.Request,
		) {

			w.Write(
				[]byte(
					"hello",
				),
			)
		},
	)

	if err := http.ListenAndServe(
		":8080",
		r,
	); err != nil {
		panic(err)
	}
}
