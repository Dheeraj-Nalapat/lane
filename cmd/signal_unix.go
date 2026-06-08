//go:build !windows

package cmd

import "syscall"

// terminate sends SIGTERM to a process for a graceful stop on Unix.
func terminate(pid int) {
	_ = syscall.Kill(pid, syscall.SIGTERM)
}
