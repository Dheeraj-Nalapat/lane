// Package lockfile is a portable advisory lock via O_CREATE|O_EXCL, with a
// stale-lock reclaim. Used to serialize shared-infra bring-up across parallel
// `lane up` invocations.
package lockfile

import (
	"fmt"
	"os"
	"time"
)

const staleAfter = 30 * time.Second

// Acquire blocks until it can create path exclusively or timeout elapses.
// Returns a release func that removes the lock.
func Acquire(path string, timeout time.Duration) (func(), error) {
	deadline := time.Now().Add(timeout)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if fi, statErr := os.Stat(path); statErr == nil && time.Since(fi.ModTime()) > staleAfter {
			_ = os.Remove(path) // reclaim a stale lock (owner likely died)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("could not acquire lock %s (held by another lane process)", path)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
