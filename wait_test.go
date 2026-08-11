package tcpwait

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func quiet() Options {
	return Options{
		DialTimeout: 100 * time.Millisecond,
		Interval:    50 * time.Millisecond,
		Logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

func startListener(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().String()
}

func TestWaitTCPSuccess(t *testing.T) {
	addr := startListener(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := WaitTCP(ctx, []string{addr}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestWaitMultipleTCP(t *testing.T) {
	a, b := startListener(t), startListener(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := Wait(ctx, []Check{TCP(a), TCP(b)}); err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestWaitTimeoutListsFailed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := WaitWithOptions(ctx, []Check{TCP("127.0.0.1:1")}, quiet())
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
	if got := err.Error(); got == "services did not respond" {
		// should include which check
		t.Fatalf("expected failed checks in message, got %q", got)
	}
}

func TestWaitPartialFailure(t *testing.T) {
	good := startListener(t)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := WaitWithOptions(ctx, []Check{TCP(good), TCP("127.0.0.1:1")}, quiet())
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected ErrTimeout, got %v", err)
	}
}

func TestWaitNoChecks(t *testing.T) {
	err := Wait(context.Background(), nil)
	if err == nil || err.Error() != "no services provided" {
		t.Fatalf("got %v", err)
	}
}

func TestWaitHTTP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := Wait(ctx, []Check{HTTP(ts.URL + "/ready")}); err != nil {
		t.Fatalf("expected http ready, got %v", err)
	}
}

func TestWaitHTTPWrongStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	err := WaitWithOptions(ctx, []Check{HTTP(ts.URL)}, quiet())
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected timeout on 503, got %v", err)
	}
}

func TestWaitHTTPStatusCustom(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := Wait(ctx, []Check{HTTPStatus(ts.URL, 204)}); err != nil {
		t.Fatalf("expected 204 ok, got %v", err)
	}
}

func TestWaitRedis(t *testing.T) {
	// Fake Redis: read PING, write +PONG
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			buf := make([]byte, 64)
			_, _ = conn.Read(buf)
			_, _ = conn.Write([]byte("+PONG\r\n"))
			_ = conn.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := Wait(ctx, []Check{Redis(ln.Addr().String())}); err != nil {
		t.Fatalf("redis: %v", err)
	}
}

func TestWaitPostgresAuth(t *testing.T) {
	// Fake PG: after startup, send AuthenticationOk (R, method 0)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// read startup (discard)
			hdr := make([]byte, 4)
			if _, err := io.ReadFull(conn, hdr); err != nil {
				_ = conn.Close()
				continue
			}
			n := int(hdr[0])<<24 | int(hdr[1])<<16 | int(hdr[2])<<8 | int(hdr[3])
			rest := make([]byte, n-4)
			_, _ = io.ReadFull(conn, rest)

			// AuthenticationOk: 'R' + len=8 + int32(0)
			msg := []byte{'R', 0, 0, 0, 8, 0, 0, 0, 0}
			_, _ = conn.Write(msg)
			_ = conn.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := Wait(ctx, []Check{Postgres(ln.Addr().String())}); err != nil {
		t.Fatalf("postgres: %v", err)
	}
}

func TestWaitBecomesReadyLater(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	go func() {
		time.Sleep(150 * time.Millisecond)
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return
		}
		defer ln.Close()
		time.Sleep(2 * time.Second)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := WaitWithOptions(ctx, []Check{TCP(addr)}, quiet()); err != nil {
		t.Fatalf("late bind: %v", err)
	}
}

func TestOptionsDefaults(t *testing.T) {
	opts := Options{}.withDefaults()
	if opts.DialTimeout != time.Second || opts.Interval != time.Second {
		t.Fatalf("defaults: %+v", opts)
	}
	if opts.Logger == nil {
		t.Fatal("logger nil")
	}
}

func TestInvalidCheck(t *testing.T) {
	err := Wait(context.Background(), []Check{{Kind: KindTCP, Target: ""}})
	if err == nil {
		t.Fatal("expected validate error")
	}
}

func TestWaitFile(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/ready"

	// Not present yet — short timeout should fail.
	ctxFail, cancelFail := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancelFail()
	if err := WaitWithOptions(ctxFail, []Check{File(path)}, quiet()); !errors.Is(err, ErrTimeout) {
		t.Fatalf("expected timeout before create, got %v", err)
	}

	// Create later.
	go func() {
		time.Sleep(100 * time.Millisecond)
		if err := os.WriteFile(path, []byte("ok"), 0o644); err != nil {
			t.Logf("write: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := WaitWithOptions(ctx, []Check{File(path)}, quiet()); err != nil {
		t.Fatalf("file ready: %v", err)
	}
}

func TestProbeFileExists(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/marker"
	if err := probeFile(path); err == nil {
		t.Fatal("expected missing file error")
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := probeFile(path); err != nil {
		t.Fatalf("expected exist ok: %v", err)
	}
}
