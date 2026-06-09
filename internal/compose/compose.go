// Package compose reads minimal structure from a base docker-compose file.
package compose

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type svc struct {
	Build yaml.Node `yaml:"build"`
}

type file struct {
	Services map[string]svc `yaml:"services"`
}

func parse(path string) (file, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return file{}, fmt.Errorf("reading compose %s: %w", path, err)
	}
	var f file
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return file{}, fmt.Errorf("parsing compose %s: %w", path, err)
	}
	return f, nil
}

// ServiceNames returns the service keys declared in the compose file at path.
func ServiceNames(path string) ([]string, error) {
	f, err := parse(path)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(f.Services))
	for k := range f.Services {
		names = append(names, k)
	}
	return names, nil
}

// BuiltServices returns the services that declare a `build:` section.
func BuiltServices(path string) ([]string, error) {
	f, err := parse(path)
	if err != nil {
		return nil, err
	}
	var built []string
	for k, s := range f.Services {
		if s.Build.Kind != 0 {
			built = append(built, k)
		}
	}
	return built, nil
}
