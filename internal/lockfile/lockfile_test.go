package lockfile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquire_MutualExclusion(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.lock")
	rel, err := Acquire(p, time.Second)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if _, err := Acquire(p, 300*time.Millisecond); err == nil {
		t.Fatal("second acquire should fail while held")
	}
	rel()
	rel2, err := Acquire(p, time.Second)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	rel2()
}

// TestAcquire_NoDeadlock: a held lock must time out (not hang), and the release
// always frees it for the next caller.
func TestAcquire_NoDeadlock(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.lock")
	rel, _ := Acquire(p, time.Second)
	done := make(chan error, 1)
	go func() { _, e := Acquire(p, 400*time.Millisecond); done <- e }()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("contended acquire should have timed out, not succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Acquire hung past its timeout (deadlock)")
	}
	rel()
}

// TestAcquire_StaleReclaim: a lock older than the stale window is reclaimed,
// so a crashed owner never locks everyone out forever.
func TestAcquire_StaleReclaim(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.lock")
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * staleAfter)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatal(err)
	}
	rel, err := Acquire(p, time.Second)
	if err != nil {
		t.Fatalf("should reclaim a stale lock: %v", err)
	}
	rel()
}
