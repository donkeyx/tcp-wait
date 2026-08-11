// Package tcpwait waits until network endpoints or paths are ready.
//
// # Overview
//
// Use [Wait] / [WaitWithOptions] to block until every [Check] succeeds (typical
// container pre-start / initContainer). Use [Watch] / [WatchWithOptions] to
// keep probing and log up/down/blip transitions (upgrade windows, firewall
// opens) until the context is cancelled.
//
// Supported probe kinds (prefer constructors over filling [Check] by hand):
//
//   - [TCP] — host:port accepts a connection
//   - [HTTP] / [HTTPStatus] — GET returns an expected status (default 200)
//   - [Redis] — RESP PING → PONG
//   - [Postgres] — StartupMessage answered (auth or non-startup error)
//   - [File] — filesystem path exists
//
// # CLI
//
// A command-line tool is provided at github.com/donkeyx/tcp-wait/v2/cmd/tcp-wait:
//
//	go install github.com/donkeyx/tcp-wait/v2/cmd/tcp-wait@latest
//
// # Errors
//
// Wait returns an error wrapping [ErrTimeout] when the context deadline fires
// before all checks pass. Use errors.Is(err, tcpwait.ErrTimeout). The message
// lists checks that never succeeded.
//
// Watch returns ctx.Err() when stopped (context.Canceled or DeadlineExceeded)
// and a [WatchResult] with the last known state of each check.
package tcpwait
