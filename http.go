package tcpwait

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

func probeHTTP(ctx context.Context, rawURL string, expectStatus int, timeout time.Duration) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: timeout}
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
