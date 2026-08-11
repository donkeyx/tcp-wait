package tcpwait

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"time"
)

// probeRedis sends PING and expects a PONG (+PONG or bulk "PONG").
func probeRedis(ctx context.Context, addr string, timeout time.Duration) error {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	if err := conn.SetDeadline(deadline); err != nil {
		return err
	}

	// RESP: *1\r\n$4\r\nPING\r\n
	if _, err := conn.Write([]byte("*1\r\n$4\r\nPING\r\n")); err != nil {
		return err
	}

	br := bufio.NewReader(conn)
	line, err := br.ReadString('\n')
	if err != nil {
		return err
	}
	line = strings.TrimSpace(line)

	switch {
	case line == "+PONG":
		return nil
	case strings.HasPrefix(line, "-"):
		return fmt.Errorf("redis error: %s", strings.TrimPrefix(line, "-"))
	case strings.HasPrefix(line, "$"):
		// bulk string: next line is payload
		payload, err := br.ReadString('\n')
		if err != nil {
			return err
		}
		if strings.TrimSpace(payload) == "PONG" {
			return nil
		}
		return fmt.Errorf("unexpected redis bulk: %q", strings.TrimSpace(payload))
	default:
		return fmt.Errorf("unexpected redis reply: %q", line)
	}
}
