package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tcpwait "github.com/donkeyx/tcp-wait/v2"
)

// version info set with ldflags
var (
	Version = "undefined"
	GitHash = "undefined"
)

func printVersion() {
	fmt.Printf("Version %s\n", Version)
	fmt.Printf("Git_Hash %s\n", GitHash)
}

func newLogger(format string, quiet bool, w io.Writer) *slog.Logger {
	level := slog.LevelInfo
	if quiet {
		level = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(w, opts)
	} else {
		handler = slog.NewJSONHandler(w, opts)
	}
	return slog.New(handler)
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

type headerFlags map[string]string

func (h *headerFlags) String() string { return "" }

func (h *headerFlags) Set(value string) error {
	name, headerValue, ok := strings.Cut(value, ":")
	name = strings.TrimSpace(name)
	if !ok || name == "" {
		return fmt.Errorf("HTTP header must use 'Name: value' format")
	}
	if *h == nil {
		*h = make(headerFlags)
	}
	(*h)[name] = strings.TrimSpace(headerValue)
	return nil
}

func buildChecks(tcpList, httpList string, httpStatus int, httpHeaders map[string]string, redisList, pgList, fileList string) ([]tcpwait.Check, error) {
	var checks []tcpwait.Check
	for _, a := range splitCSV(tcpList) {
		checks = append(checks, tcpwait.TCP(a))
	}
	for _, u := range splitCSV(httpList) {
		checks = append(checks, tcpwait.HTTPStatusWithHeaders(u, httpStatus, httpHeaders))
	}
	for _, a := range splitCSV(redisList) {
		checks = append(checks, tcpwait.Redis(a))
	}
	for _, a := range splitCSV(pgList) {
		checks = append(checks, tcpwait.Postgres(a))
	}
	for _, p := range splitCSV(fileList) {
		checks = append(checks, tcpwait.File(p))
	}
	if len(checks) == 0 {
		return nil, fmt.Errorf("no checks: set at least one of -hp/-tcp, -http, -redis, -postgres, -file")
	}
	return checks, nil
}

// remainingCommand returns argv after flags, stripping a leading "--" if present.
func remainingCommand() []string {
	args := flag.Args()
	if len(args) == 0 {
		return nil
	}
	if args[0] == "--" {
		return args[1:]
	}
	return args
}

func execCommand(argv []string) error {
	if len(argv) == 0 {
		return fmt.Errorf("empty command")
	}
	bin, err := exec.LookPath(argv[0])
	if err != nil {
		return err
	}
	// Replace this process so the child gets signals / PID 1 semantics in containers.
	return syscall.Exec(bin, argv, os.Environ())
}

// rootContext is cancelled on SIGINT/SIGTERM, and optionally by -t seconds (0 = no deadline).
func rootContext(timeoutSec int) (context.Context, context.CancelFunc) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	if timeoutSec <= 0 {
		return ctx, stop
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	return ctx, func() {
		cancel()
		stop()
	}
}

func main() {
	var (
		tcpHosts       string
		httpURLs       string
		redisHosts     string
		postgresHosts  string
		filePaths      string
		timeout        int
		dial           time.Duration
		interval       time.Duration
		httpStatus     int
		httpHeaders    headerFlags
		tlsEnabled     bool
		tlsServerName  string
		tlsInsecure    bool
		displayVersion bool
		output         string
		quiet          bool
		watch          bool
	)

	flag.BoolVar(&displayVersion, "version", false, "version information")
	flag.IntVar(&timeout, "t", 20, "overall timeout in seconds (0 = no limit; useful with -watch)")
	flag.DurationVar(&dial, "dial", time.Second, "per-attempt dial/probe timeout")
	flag.DurationVar(&interval, "interval", time.Second, "delay between probe attempts")
	flag.StringVar(&tcpHosts, "hp", "", "comma-separated host:port list (TCP); alias of -tcp")
	flag.StringVar(&tcpHosts, "tcp", "", "comma-separated host:port list (TCP)")
	flag.StringVar(&httpURLs, "http", "", "comma-separated HTTP(S) URLs (GET, expect 200)")
	flag.IntVar(&httpStatus, "http-status", 200, "expected HTTP status for all -http URLs")
	flag.Var(&httpHeaders, "http-header", "HTTP request header (repeatable, Name: value)")
	flag.StringVar(&redisHosts, "redis", "", "comma-separated host:port list (Redis PING)")
	flag.StringVar(&postgresHosts, "postgres", "", "comma-separated host:port list (Postgres startup probe)")
	flag.StringVar(&filePaths, "file", "", "comma-separated filesystem paths that must exist")
	flag.StringVar(&output, "o", "json", "log format: json or text")
	flag.BoolVar(&quiet, "q", false, "quiet: only log errors")
	flag.BoolVar(&watch, "watch", false, "watch mode: log up/down/blip transitions until timeout or signal")
	flag.BoolVar(&watch, "w", false, "short for -watch")
	flag.BoolVar(&tlsEnabled, "tls", false, "use TLS for TCP/Redis probes and configure HTTPS verification")
	flag.StringVar(&tlsServerName, "tls-server-name", "", "TLS server name override")
	flag.BoolVar(&tlsInsecure, "tls-insecure-skip-verify", false, "disable TLS certificate verification")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] [--] [command [args...]]\n\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "Wait until checks pass, or -watch for continuous up/down/blip logging.\n\n")
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), `
Examples:
  tcp-wait -hp db:5432,redis:6379 -t 60
  tcp-wait -tcp db:5432 -http http://api:8080/readyz -t 30
  tcp-wait -redis cache:6379 -postgres db:5432 -t 60 -- /app/server
  tcp-wait -file /var/run/app.ready -t 30

  # upgrade / firewall blip monitor (Ctrl-C or -t to stop; -t 0 = until signal)
  tcp-wait -watch -postgres db:5432 -redis cache:6379 -t 0 -interval 500ms -o text
  tcp-wait -w -tcp bastion:5432 -t 600 -interval 1s -o text
`)
	}
	flag.Parse()

	log := newLogger(output, quiet, os.Stdout)

	if displayVersion {
		printVersion()
		os.Exit(0)
	}

	checks, err := buildChecks(tcpHosts, httpURLs, httpStatus, httpHeaders, redisHosts, postgresHosts, filePaths)
	if err != nil {
		flag.Usage()
		os.Exit(1)
	}

	cmd := remainingCommand()
	if watch && len(cmd) > 0 {
		log.Error("cannot combine -watch with a command to exec")
		os.Exit(1)
	}

	ctx, cancel := rootContext(timeout)
	defer cancel()

	opts := tcpwait.Options{
		DialTimeout:           dial,
		Interval:              interval,
		Logger:                log,
		TLS:                   tlsEnabled,
		TLSServerName:         tlsServerName,
		TLSInsecureSkipVerify: tlsInsecure,
	}

	if watch {
		runWatch(ctx, cancel, checks, opts, log)
		return
	}

	if err := tcpwait.WaitWithOptions(ctx, checks, opts); err != nil {
		cancel()
		labels := checkLabels(checks)
		switch {
		case errors.Is(err, context.Canceled):
			log.Error("interrupted")
			os.Exit(130)
		case errors.Is(err, tcpwait.ErrTimeout):
			log.Error(err.Error(), "status", "timeout", "checks", labels)
			os.Exit(1)
		default:
			log.Error(err.Error(), "status", "error", "checks", labels)
			os.Exit(1)
		}
	}

	labels := checkLabels(checks)
	log.Info("services are ready!", "status", "ready", "checks", labels)

	if len(cmd) == 0 {
		os.Exit(0)
	}

	if err := execCommand(cmd); err != nil {
		log.Error("exec failed", "err", err.Error())
		os.Exit(1)
	}
}

func checkLabels(checks []tcpwait.Check) []string {
	labels := make([]string, len(checks))
	for i, c := range checks {
		labels[i] = c.String()
	}
	return labels
}

func runWatch(ctx context.Context, cancel context.CancelFunc, checks []tcpwait.Check, opts tcpwait.Options, log *slog.Logger) {
	log.Info("watch started", "checks", len(checks))
	res, err := tcpwait.WatchWithOptions(ctx, checks, opts)
	cancel()

	// Always print a short summary on the way out.
	if res.AllUp() {
		log.Info("watch stopped", "status", "all up", "reason", ctxReason(err))
	} else {
		log.Warn("watch stopped", "status", "degraded", "down", res.Down(), "reason", ctxReason(err))
	}

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		if res.AllUp() {
			os.Exit(0)
		}
		os.Exit(1)
	case errors.Is(err, context.Canceled):
		// Ctrl-C / SIGTERM — success from an ops "I saw enough" POV
		os.Exit(0)
	default:
		if err != nil {
			log.Error(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	}
}

func ctxReason(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "signal"
	case err != nil:
		return err.Error()
	default:
		return "done"
	}
}
