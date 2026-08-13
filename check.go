package tcpwait

import (
	"context"
	"fmt"
	"strings"
)

// Kind identifies how a Check probes readiness.
type Kind string

const (
	KindTCP      Kind = "tcp"
	KindHTTP     Kind = "http"
	KindRedis    Kind = "redis"
	KindPostgres Kind = "postgres"
	KindFile     Kind = "file"
)

// Check is a single readiness probe.
// Prefer constructors (TCP, HTTP, Redis, Postgres, File) over filling the struct by hand.
type Check struct {
	Kind   Kind
	Target string // host:port for tcp/redis/postgres; URL for http; path for file

	// HTTP: expected status code (default 200).
	ExpectStatus int
	// HTTP request headers.
	Headers map[string]string

	// Postgres: StartupMessage user/database (defaults: postgres/postgres).
	Username string
	Database string
}

// TCP waits until addr (host:port) accepts a TCP connection.
func TCP(addr string) Check {
	return Check{Kind: KindTCP, Target: addr}
}

// HTTP waits until GET url returns ExpectStatus (default 200).
func HTTP(url string) Check {
	return Check{Kind: KindHTTP, Target: url, ExpectStatus: 200}
}

// HTTPWithHeaders waits until GET url returns 200 with the supplied headers.
func HTTPWithHeaders(url string, headers map[string]string) Check {
	return HTTPStatusWithHeaders(url, 200, headers)
}

// HTTPStatus is like HTTP but with an explicit expected status code.
func HTTPStatus(url string, status int) Check {
	return Check{Kind: KindHTTP, Target: url, ExpectStatus: status}
}

// HTTPStatusWithHeaders is like HTTPStatus with request headers.
func HTTPStatusWithHeaders(url string, status int, headers map[string]string) Check {
	return Check{Kind: KindHTTP, Target: url, ExpectStatus: status, Headers: cloneHeaders(headers)}
}

func cloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for key, value := range headers {
		out[key] = value
	}
	return out
}

// Redis waits until a Redis PING receives PONG at addr (host:port).
func Redis(addr string) Check {
	return Check{Kind: KindRedis, Target: addr}
}

// Postgres waits until PostgreSQL at addr answers a StartupMessage
// (auth request or a non-"starting up" error). Optional user/database
// default to "postgres".
func Postgres(addr string, userDB ...string) Check {
	c := Check{Kind: KindPostgres, Target: addr, Username: "postgres", Database: "postgres"}
	if len(userDB) > 0 && userDB[0] != "" {
		c.Username = userDB[0]
	}
	if len(userDB) > 1 && userDB[1] != "" {
		c.Database = userDB[1]
	}
	return c
}

// File waits until path exists (regular file, socket, directory, etc.).
// Useful for readiness marker files or unix socket paths.
func File(path string) Check {
	return Check{Kind: KindFile, Target: path}
}

// String returns a stable id for logs and timeout errors (e.g. "tcp://db:5432").
func (c Check) String() string {
	// HTTP targets are already full URLs; don't prefix kind://
	if c.Kind == KindHTTP {
		return c.Target
	}
	// file:///abs/path  (path usually starts with /)
	if c.Kind == KindFile {
		return "file://" + c.Target
	}
	return fmt.Sprintf("%s://%s", c.Kind, c.Target)
}

func (c Check) label() string { return c.String() }

func (c Check) validate() error {
	if c.Target == "" {
		return fmt.Errorf("empty target for kind %q", c.Kind)
	}
	switch c.Kind {
	case KindTCP, KindHTTP, KindRedis, KindPostgres, KindFile:
		return nil
	case "":
		return fmt.Errorf("missing check kind for target %q", c.Target)
	default:
		return fmt.Errorf("unknown check kind %q", c.Kind)
	}
}

// probe runs one attempt. Returns nil on success.
func (c Check) probe(ctx context.Context, opts Options) error {
	if err := c.validate(); err != nil {
		return err
	}
	switch c.Kind {
	case KindTCP:
		return probeTCP(ctx, c.Target, opts)
	case KindHTTP:
		status := c.ExpectStatus
		if status == 0 {
			status = 200
		}
		return probeHTTP(ctx, c, status, opts)
	case KindRedis:
		return probeRedis(ctx, c.Target, opts)
	case KindPostgres:
		user := c.Username
		if user == "" {
			user = "postgres"
		}
		db := c.Database
		if db == "" {
			db = "postgres"
		}
		return probePostgres(ctx, c.Target, user, db, opts.DialTimeout)
	case KindFile:
		return probeFile(c.Target)
	default:
		return fmt.Errorf("unknown check kind %q", c.Kind)
	}
}

// TCPAddrs is a convenience: one TCP check per host:port.
func TCPAddrs(addrs ...string) []Check {
	out := make([]Check, 0, len(addrs))
	for _, a := range addrs {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		out = append(out, TCP(a))
	}
	return out
}
