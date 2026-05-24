package main

import (
	"context"
	"fmt"
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

	r := newRedisLimiter(
		"localhost:6379",
		5,
		1,
	)

	for i := 0; i < 10; i++ {

		d, err := r.Decide(
			context.Background(),
			"192.168.1.1",
		)

		if err != nil {
			panic(err)
		}

		fmt.Println(
			d.Allowed,
			d.Remaining,
			d.RetryAfter,
		)
	}
}
