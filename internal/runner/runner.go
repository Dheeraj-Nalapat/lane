// Package runner brings a stack up via a selected backend (Tilt or Compose).
package runner

import (
	"fmt"

	"github.com/dheeraj-nalapat/lane/internal/override"
)

// RunSpec is everything a runner needs to bring one stack up.
type RunSpec struct {
	Slug         string
	Dir          string
	ComposePath  string
	OverridePath string
	Routes       []override.Route
	Detach       bool
	Build        bool
	TiltPort     int    // 0 when not Tilt
	DynamicPath  string // tilt-UI route file; "" when not Tilt
	Env          []string
}

// Runner brings a stack up. Teardown stays shared in cmd/down.go.
type Runner interface {
	Up(RunSpec) error
	DryRunLines(RunSpec) string
	Name() string
}

// Select returns "tilt" or "compose" from the manifest hint and detection.
func Select(manifestRunner string, tiltfilePresent bool) string {
	switch manifestRunner {
	case "tilt", "compose":
		return manifestRunner
	}
	if tiltfilePresent {
		return "tilt"
	}
	return "compose"
}

// New constructs the runner for a selected name (defaults to compose).
func New(name string) Runner {
	if name == "tilt" {
		return tiltRunner{}
	}
	return composeRunner{}
}

// printURLs prints the shared app-route URLs for a stack.
func printURLs(s RunSpec) {
	fmt.Printf("lane: %s\n", s.Slug)
	for _, r := range s.Routes {
		fmt.Printf("  → http://%s  (%s:%d)\n", r.Hostname, r.Service, r.Port)
	}
}
