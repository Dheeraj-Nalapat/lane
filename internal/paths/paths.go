// Package paths centralizes berth's on-disk layout under ~/.berth.
package paths

import (
	"os"
	"path/filepath"
)

// Home is BERTH_HOME or ~/.berth.
func Home() string {
	if h := os.Getenv("BERTH_HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".berth")
}

func Overrides() string      { return filepath.Join(Home(), "overrides") }
func Run() string            { return filepath.Join(Home(), "run") }
func Traefik() string        { return filepath.Join(Home(), "traefik") }
func TraefikDynamic() string { return filepath.Join(Traefik(), "dynamic") }

// Ensure creates all berth directories if missing.
func Ensure() error {
	for _, d := range []string{Overrides(), Run(), TraefikDynamic()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
