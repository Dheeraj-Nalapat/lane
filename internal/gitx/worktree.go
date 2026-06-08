// Package gitx provides git worktree helpers for slug derivation.
package gitx

import (
	"os/exec"
	"path/filepath"
	"strings"
)

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// Worktree reports whether dir is inside a *linked* git worktree, and if so its
// name (the leaf of .git/worktrees/<name>). For the main checkout, a
// non-git dir, or any error treated as "not a repo", it returns ("", false, nil).
func Worktree(dir string) (name string, linked bool, err error) {
	gitDir, err := run(dir, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", false, nil // not a git repo
	}
	commonDir, err := run(dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", false, err
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(dir, commonDir)
	}
	gitDir = filepath.Clean(gitDir)
	commonDir = filepath.Clean(commonDir)
	if gitDir == commonDir {
		return "", false, nil // main checkout
	}
	return filepath.Base(gitDir), true, nil
}
