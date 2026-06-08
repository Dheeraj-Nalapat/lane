// Package compose reads minimal structure from a base docker-compose file.
package compose

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type file struct {
	Services map[string]yaml.Node `yaml:"services"`
}

// ServiceNames returns the service keys declared in the compose file at path.
func ServiceNames(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading compose %s: %w", path, err)
	}
	var f file
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("parsing compose %s: %w", path, err)
	}
	names := make([]string, 0, len(f.Services))
	for k := range f.Services {
		names = append(names, k)
	}
	return names, nil
}
