// Package manifest loads the committed .lane.toml project descriptor.
package manifest

import (
	"errors"
	"fmt"

	"github.com/BurntSushi/toml"
)

// Route declares one web entrypoint to route through Traefik.
type Route struct {
	Service string `toml:"service"` // compose service name
	Port    int    `toml:"port"`    // internal container port
	Host    string `toml:"host"`    // optional host template, default "{slug}"
}

// Manifest is the parsed .lane.toml.
type Manifest struct {
	Name        string  `toml:"name"`         // base slug
	ComposeFile string  `toml:"compose_file"` // path to base compose, relative to project dir
	APITarget   string  `toml:"api_target"`   // optional, e.g. "server:8000" for dev-server /api proxying
	Runner      string  `toml:"runner"`       // "", "tilt", or "compose" (auto if "")
	Routes      []Route `toml:"routes"`
}

// Load reads and validates a .lane.toml at path.
func Load(path string) (*Manifest, error) {
	var m Manifest
	if _, err := toml.DecodeFile(path, &m); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if m.Name == "" {
		return nil, errors.New(".lane.toml: 'name' is required")
	}
	if m.ComposeFile == "" {
		return nil, errors.New(".lane.toml: 'compose_file' is required")
	}
	if len(m.Routes) == 0 {
		return nil, errors.New(".lane.toml: at least one [[routes]] entry is required")
	}
	if m.Runner != "" && m.Runner != "tilt" && m.Runner != "compose" {
		return nil, fmt.Errorf(".lane.toml: runner must be \"tilt\" or \"compose\" (got %q)", m.Runner)
	}
	for i := range m.Routes {
		if m.Routes[i].Host == "" {
			m.Routes[i].Host = "{slug}"
		}
		if m.Routes[i].Service == "" || m.Routes[i].Port == 0 {
			return nil, fmt.Errorf(".lane.toml: route %d needs both 'service' and 'port'", i)
		}
	}
	return &m, nil
}
