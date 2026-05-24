package main

import (
	"context"
	"fmt"

	"rate-limiter/internal/limiter"
)

func main() {

	l := limiter.NewLocalLimiter(
		10,
		2,
	)

	for i := 0; i < 15; i++ {

		d, err := l.Decide(
			context.Background(),
			"192.168.1.1",
		)

		if err != nil {
			panic(err)
		}

		fmt.Println(
			d.Allowed,
			d.Remaining,
		)
	}
}
