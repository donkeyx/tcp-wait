package tcpwait

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrTimeout is returned when the context deadline is hit before all checks pass.
// Use errors.Is(err, ErrTimeout). The error string lists checks that never succeeded.
var ErrTimeout = errors.New("services did not respond")

// Wait runs all checks in parallel until each succeeds or ctx is done.
// Defaults: 1s dial timeout, 1s retry interval, no logging.
func Wait(ctx context.Context, checks []Check) error {
	return WaitWithOptions(ctx, checks, Options{})
}

// WaitWithOptions is like Wait with explicit dial/retry/logging options.
func WaitWithOptions(ctx context.Context, checks []Check, opts Options) error {
	opts = opts.withDefaults()

	if len(checks) == 0 {
		return fmt.Errorf("no services provided")
	}
	for _, c := range checks {
		if err := c.validate(); err != nil {
			return err
		}
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		pending = make(map[string]struct{}, len(checks))
	)
	for _, c := range checks {
		pending[c.label()] = struct{}{}
	}

	done := make(chan struct{})
	wg.Add(len(checks))

	for _, c := range checks {
		go func(c Check) {
			defer wg.Done()
			label := c.label()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				err := c.probe(ctx, opts)
				if err == nil {
					mu.Lock()
					delete(pending, label)
					mu.Unlock()
					opts.Logger.Info("check ready", "check", label)
					return
				}

				// Context cancelled mid-probe: stop quietly.
				if ctx.Err() != nil {
					return
				}

				opts.Logger.Warn("check failed", "check", label, "err", err.Error())

				select {
				case <-ctx.Done():
					return
				case <-time.After(opts.Interval):
				}
			}
		}(c)
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		if err := ctx.Err(); err != nil {
			return waitCtxErr(ctx, pendingSnapshot(&mu, pending))
		}
		return nil
	case <-ctx.Done():
		// Give workers a brief moment to exit; then report who never came up.
		<-done
		return waitCtxErr(ctx, pendingSnapshot(&mu, pending))
	}
}

func waitCtxErr(ctx context.Context, failed []string) error {
	// SIGINT/SIGTERM → Canceled; keep that distinct from a deadline timeout.
	if err := ctx.Err(); errors.Is(err, context.Canceled) {
		return err
	}
	return timeoutErr(failed)
}

func pendingSnapshot(mu *sync.Mutex, pending map[string]struct{}) []string {
	mu.Lock()
	defer mu.Unlock()
	out := make([]string, 0, len(pending))
	for k := range pending {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func timeoutErr(failed []string) error {
	if len(failed) == 0 {
		return ErrTimeout
	}
	return fmt.Errorf("%w: %s", ErrTimeout, strings.Join(failed, ", "))
}

// WaitTCP waits until each host:port accepts a TCP connection.
// Convenience wrapper around Wait + TCPAddrs.
func WaitTCP(ctx context.Context, addrs []string) error {
	return Wait(ctx, TCPAddrs(addrs...))
}

// WaitTCPWithOptions is WaitTCP with options.
func WaitTCPWithOptions(ctx context.Context, addrs []string, opts Options) error {
	return WaitWithOptions(ctx, TCPAddrs(addrs...), opts)
}
