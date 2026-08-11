package tcpwait

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// WatchResult is the last known state of each check when Watch stops.
type WatchResult struct {
	// Up maps check label (Check.String()) → last successful probe.
	Up map[string]bool
}

// AllUp reports whether every check was up on the last probe.
func (r WatchResult) AllUp() bool {
	if len(r.Up) == 0 {
		return false
	}
	for _, up := range r.Up {
		if !up {
			return false
		}
	}
	return true
}

// Down returns labels that were down on the last probe (sorted).
func (r WatchResult) Down() []string {
	var out []string
	for label, up := range r.Up {
		if !up {
			out = append(out, label)
		}
	}
	sort.Strings(out)
	return out
}

// Watch repeatedly probes all checks until ctx is cancelled.
// It logs only state transitions:
//
//	INFO  "check up"   — first success, or recovery after down
//	WARN  "check down" — first failure while never up yet
//	WARN  "check blip" — transition from up → down (the upgrade hole)
//
// Defaults match Wait (1s dial, 1s interval, quiet logger).
// The returned error is usually ctx.Err() (Canceled / DeadlineExceeded).
func Watch(ctx context.Context, checks []Check) (WatchResult, error) {
	return WatchWithOptions(ctx, checks, Options{})
}

// WatchWithOptions is Watch with explicit options.
func WatchWithOptions(ctx context.Context, checks []Check, opts Options) (WatchResult, error) {
	opts = opts.withDefaults()
	result := WatchResult{Up: make(map[string]bool, len(checks))}

	if len(checks) == 0 {
		return result, fmt.Errorf("no services provided")
	}
	for _, c := range checks {
		if err := c.validate(); err != nil {
			return result, err
		}
	}

	var (
		wg sync.WaitGroup
		mu sync.Mutex
	)

	wg.Add(len(checks))
	for _, c := range checks {
		go func(c Check) {
			defer wg.Done()
			label := c.label()
			var (
				haveState bool
				lastUp    bool
			)

			for {
				if ctx.Err() != nil {
					return
				}

				err := c.probe(ctx, opts)
				up := err == nil

				// Context killed mid-probe: don't emit a bogus transition.
				if ctx.Err() != nil {
					return
				}

				if !haveState || up != lastUp {
					if up {
						opts.Logger.Info("check up", "check", label)
					} else {
						msg := "check down"
						if haveState && lastUp {
							msg = "check blip"
						}
						errStr := ""
						if err != nil {
							errStr = err.Error()
						}
						opts.Logger.Warn(msg, "check", label, "err", errStr)
					}
					haveState = true
					lastUp = up

					mu.Lock()
					result.Up[label] = up
					mu.Unlock()
				}

				select {
				case <-ctx.Done():
					return
				case <-time.After(opts.Interval):
				}
			}
		}(c)
	}

	wg.Wait()

	mu.Lock()
	final := WatchResult{Up: make(map[string]bool, len(result.Up))}
	for k, v := range result.Up {
		final.Up[k] = v
	}
	mu.Unlock()

	err := ctx.Err()
	if err == nil {
		err = context.Canceled
	}
	return final, err
}
