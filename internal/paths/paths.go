// Package paths centralizes lane's on-disk layout under ~/.lane.
package paths

import (
	"os"
	"path/filepath"
)

// Home is LANE_HOME or ~/.lane.
func Home() string {
	if h := os.Getenv("LANE_HOME"); h != "" {
		return h
	}
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".lane")
}

func Overrides() string      { return filepath.Join(Home(), "overrides") }
func Run() string            { return filepath.Join(Home(), "run") }
func Traefik() string        { return filepath.Join(Home(), "traefik") }
func TraefikDynamic() string { return filepath.Join(Traefik(), "dynamic") }

// Ensure creates all lane directories if missing.
func Ensure() error {
	for _, d := range []string{Overrides(), Run(), TraefikDynamic()} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
