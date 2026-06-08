// Package scaffold powers `berth init`: guess the web entrypoint and render
// a starter .berth.toml.
package scaffold

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

type svc struct {
	Ports []string `yaml:"ports"`
}
type composeDoc struct {
	Services map[string]svc `yaml:"services"`
}

// GuessWebEntry picks the most likely web service+port from compose text:
// prefer services named ui/web/frontend, else one publishing :80/:5173/:3000.
func GuessWebEntry(composeYAML string) (string, int) {
	var d composeDoc
	if yaml.Unmarshal([]byte(composeYAML), &d) != nil {
		return "", 0
	}
	names := make([]string, 0, len(d.Services))
	for n := range d.Services {
		names = append(names, n)
	}
	sort.Strings(names)

	preferred := map[string]bool{"ui": true, "web": true, "frontend": true}
	webPorts := map[int]bool{80: true, 5173: true, 3000: true, 8080: true}

	containerPort := func(s svc) int {
		for _, p := range s.Ports {
			// "host:container" → container side
			cp := p
			if i := indexByte(p, ':'); i >= 0 {
				cp = p[i+1:]
			}
			n := atoi(cp)
			if n > 0 {
				return n
			}
		}
		return 0
	}

	for _, n := range names {
		if preferred[n] {
			if p := containerPort(d.Services[n]); p > 0 {
				return n, p
			}
		}
	}
	for _, n := range names {
		if p := containerPort(d.Services[n]); webPorts[p] {
			return n, p
		}
	}
	return "", 0
}

// RenderManifest produces .berth.toml content.
func RenderManifest(name, composeFile, service string, port int) string {
	return fmt.Sprintf(`name = "%s"
compose_file = "%s"

[[routes]]
service = "%s"
port = %d
# host = "{slug}"   # default; use "api.{slug}" for a second route
`, name, composeFile, service, port)
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
