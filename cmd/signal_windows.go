//go:build windows

package cmd

import "os"

// terminate stops a process on Windows (no SIGTERM; hard kill).
func terminate(pid int) {
	if p, err := os.FindProcess(pid); err == nil {
		_ = p.Kill()
	}
}
