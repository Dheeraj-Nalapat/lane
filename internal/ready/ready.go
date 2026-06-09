// Package ready polls routed URLs until a stack is serving HTTP.
package ready

import (
	"fmt"
	"net/http"
	"time"
)

// WaitReady blocks until every URL returns an HTTP response with status < 500
// (i.e. the route exists and the backend is up — not a Traefik 502/503), or the
// timeout elapses. A nil client gets a default 3s-per-request client.
func WaitReady(urls []string, timeout time.Duration, client *http.Client) error {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	deadline := time.Now().Add(timeout)
	for _, u := range urls {
		for {
			if probe(client, u) {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out waiting for %s to become ready", u)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}

func probe(client *http.Client, u string) bool {
	resp, err := client.Get(u)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode < 500
}
