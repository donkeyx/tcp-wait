package tcpwait

import (
	"context"
	"crypto/tls"
	"net"
)

func probeTCP(ctx context.Context, addr string, opts Options) error {
	d := net.Dialer{Timeout: opts.DialTimeout}
	conn, err := dialProbe(ctx, &d, addr, opts)
	if err != nil {
		return err
	}
	return conn.Close()
}

func dialProbe(ctx context.Context, d *net.Dialer, addr string, opts Options) (net.Conn, error) {
	if !opts.TLS {
		return d.DialContext(ctx, "tcp", addr)
	}
	return (&tls.Dialer{
		NetDialer: d,
		Config: &tls.Config{
			ServerName:         opts.TLSServerName,
			InsecureSkipVerify: opts.TLSInsecureSkipVerify, // explicitly configured by the caller
		},
	}).DialContext(ctx, "tcp", addr)
}
