package tcpwait_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	tcpwait "github.com/donkeyx/tcp-wait/v2"
)

// ExampleWait shows building checks and waiting until they pass (or time out).
func ExampleWait() {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := tcpwait.Wait(ctx, []tcpwait.Check{
		// tcpwait.TCP("db:5432"),
		// tcpwait.Redis("cache:6379"),
		// tcpwait.Postgres("db:5432"),
		// tcpwait.HTTP("http://api:8080/readyz"),
		// tcpwait.File("/var/run/app.ready"),
		tcpwait.TCP("127.0.0.1:1"), // nothing listening — will time out in this example
	})
	if errors.Is(err, tcpwait.ErrTimeout) {
		fmt.Println("timed out")
		return
	}
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println("ready")
	// Output: timed out
}

// ExampleWaitTCP is a shortcut when you only need TCP accept checks.
func ExampleWaitTCP() {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	if err := tcpwait.WaitTCP(ctx, []string{"127.0.0.1:1"}); err != nil {
		fmt.Println("not ready")
		return
	}
	fmt.Println("ready")
	// Output: not ready
}

// ExampleWatch shows continuous monitoring (upgrade / firewall blips).
// Cancel the context (timeout or signal) to stop; inspect WatchResult.
func ExampleWatch() {
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	res, err := tcpwait.WatchWithOptions(ctx, []tcpwait.Check{
		tcpwait.TCP("127.0.0.1:1"),
	}, tcpwait.Options{
		Interval:    100 * time.Millisecond,
		DialTimeout: 50 * time.Millisecond,
	})
	_ = err // typically context.DeadlineExceeded
	if res.AllUp() {
		fmt.Println("ended all up")
	} else {
		fmt.Println("ended degraded")
	}
	// Output: ended degraded
}
