package tcpwait

import (
	"io"
	"log/slog"
	"time"
)

// Options configure Wait. Zero values use defaults.
type Options struct {
	// DialTimeout is how long each probe attempt may take (dial / HTTP client / etc).
	// Default: 1s.
	DialTimeout time.Duration
	// Interval is how long to wait between failed attempts for a check. Default: 1s.
	Interval time.Duration
	// Logger receives per-attempt warnings and per-check ready messages.
	// Nil discards logs.
	Logger *slog.Logger
	// TLS enables TLS for TCP and Redis probes, and configures HTTPS probes.
	TLS bool
	// TLSServerName overrides the server name used for certificate verification.
	TLSServerName string
	// TLSInsecureSkipVerify disables certificate verification. Use only for
	// trusted development or private test endpoints.
	TLSInsecureSkipVerify bool
}

func (o Options) withDefaults() Options {
	if o.DialTimeout <= 0 {
		o.DialTimeout = time.Second
	}
	if o.Interval <= 0 {
		o.Interval = time.Second
	}
	if o.Logger == nil {
		o.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return o
}
