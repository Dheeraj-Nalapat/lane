// Package ready polls routed URLs until a stack is serving HTTP.
package ready

import (
	"fmt"
	"net/http"
	"time"
)

// WaitReady blocks until every URL returns a *served* HTTP response — i.e. the
// router exists and the backend answered. It excludes Traefik's no-route 404
// (router not registered yet) and gateway 5xx (502/503/504, backend not up), so
// it doesn't return before the stack is actually reachable. Times out otherwise.
// A nil client gets a default 3s-per-request client.
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
	// 404 = Traefik has no route yet (discovery lag); 5xx = gateway/backend not
	// up. Anything else (2xx/3xx/other 4xx like auth) means a backend answered.
	return resp.StatusCode != http.StatusNotFound && resp.StatusCode < 500
}
