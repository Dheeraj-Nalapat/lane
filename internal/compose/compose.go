// Package compose reads minimal structure from a base docker-compose file.
package compose

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type svc struct {
	Build  yaml.Node   `yaml:"build"`
	Expose []yaml.Node `yaml:"expose"`
	Ports  []yaml.Node `yaml:"ports"`
}

type file struct {
	Services map[string]svc `yaml:"services"`
}

// Service is a compose service with the bits lane needs for routing.
type Service struct {
	Name  string
	Build bool
	Port  int // discovered container port; 0 if none/ambiguous
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

// Services returns every service with its build flag and discovered port.
func Services(path string) ([]Service, error) {
	f, err := parse(path)
	if err != nil {
		return nil, err
	}
	out := make([]Service, 0, len(f.Services))
	for name, s := range f.Services {
		out = append(out, Service{
			Name:  name,
			Build: s.Build.Kind != 0,
			Port:  discoverPort(s),
		})
	}
	return out, nil
}

// discoverPort returns the single container port a service exposes, or 0 when
// there is none or more than one (ambiguous). expose wins over ports.
func discoverPort(s svc) int {
	if ps := exposePorts(s.Expose); len(ps) == 1 {
		return ps[0]
	}
	if ps := targetPorts(s.Ports); len(ps) == 1 {
		return ps[0]
	}
	return 0
}

func exposePorts(nodes []yaml.Node) []int {
	var ports []int
	for _, n := range nodes {
		if p := atoiPort(n.Value); p > 0 {
			ports = append(ports, p)
		}
	}
	return ports
}

// targetPorts returns the distinct container-side ports from a `ports:` list,
// handling both short ("8000:80", "80", "127.0.0.1:8000:80/tcp") and long
// (mapping with a `target:` key) syntax.
func targetPorts(nodes []yaml.Node) []int {
	seen := map[int]bool{}
	var ports []int
	add := func(p int) {
		if p > 0 && !seen[p] {
			seen[p] = true
			ports = append(ports, p)
		}
	}
	for _, n := range nodes {
		switch n.Kind {
		case yaml.ScalarNode:
			add(shortTarget(n.Value))
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				if n.Content[i].Value == "target" {
					add(atoiPort(n.Content[i+1].Value))
				}
			}
		}
	}
	return ports
}

// shortTarget extracts the container port from short port syntax: the segment
// after the last ':' (or the whole value), minus any "/tcp" protocol suffix.
func shortTarget(v string) int {
	if i := strings.LastIndexByte(v, ':'); i >= 0 {
		v = v[i+1:]
	}
	return atoiPort(v)
}

func atoiPort(v string) int {
	if i := strings.IndexByte(v, '/'); i >= 0 { // strip "/tcp", "/udp"
		v = v[:i]
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}
