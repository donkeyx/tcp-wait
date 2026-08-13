package tcpwait

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
)

func probeHTTP(ctx context.Context, check Check, expectStatus int, opts Options) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, check.Target, nil)
	if err != nil {
		return err
	}
	for key, value := range check.Headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{Timeout: opts.DialTimeout, Transport: httpTransport(opts)}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// Drain so the connection can be reused cleanly; ignore errors.
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))

	if res.StatusCode != expectStatus {
		return fmt.Errorf("status %d (want %d)", res.StatusCode, expectStatus)
	}
	return nil
}

func httpTransport(opts Options) http.RoundTripper {
	if !opts.TLS && opts.TLSServerName == "" && !opts.TLSInsecureSkipVerify {
		return http.DefaultTransport
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		ServerName:         opts.TLSServerName,
		InsecureSkipVerify: opts.TLSInsecureSkipVerify, // explicitly configured by the caller
	}
	return transport
}
