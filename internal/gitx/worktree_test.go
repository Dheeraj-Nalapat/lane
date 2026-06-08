package gitx

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestWorktree_MainCheckoutReturnsFalse(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	name, linked, err := Worktree(dir)
	if err != nil {
		t.Fatalf("Worktree error: %v", err)
	}
	if linked {
		t.Fatalf("main checkout reported as linked worktree (name=%q)", name)
	}
}

func TestWorktree_LinkedReturnsName(t *testing.T) {
	main := t.TempDir()
	git(t, main, "init", "-q")
	git(t, main, "-c", "user.email=a@b.c", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init")
	wt := filepath.Join(t.TempDir(), "featx")
	git(t, main, "worktree", "add", "-q", wt)

	name, linked, err := Worktree(wt)
	if err != nil {
		t.Fatalf("Worktree error: %v", err)
	}
	if !linked {
		t.Fatal("linked worktree not detected")
	}
	if name != "featx" {
		t.Fatalf("worktree name = %q, want %q", name, "featx")
	}
}

func TestWorktree_NotARepo(t *testing.T) {
	_, linked, err := Worktree(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if linked {
		t.Fatal("non-repo reported as linked worktree")
	}
}
