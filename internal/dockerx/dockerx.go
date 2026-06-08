// Package dockerx queries Docker for berth-managed containers.
package dockerx

import (
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"

	"github.com/dheerajnalapat/berth/internal/stack"
)

type psLine struct {
	Names  string `json:"Names"`
	Labels string `json:"Labels"`
	State  string `json:"State"`
}

func labelMap(s string) map[string]string {
	m := map[string]string{}
	for _, kv := range strings.Split(s, ",") {
		if i := strings.IndexByte(kv, '='); i > 0 {
			m[kv[:i]] = kv[i+1:]
		}
	}
	return m
}

func parsePS(out []byte) []stack.Stack {
	bySlug := map[string]*stack.Stack{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var p psLine
		if json.Unmarshal([]byte(line), &p) != nil {
			continue
		}
		lbl := labelMap(p.Labels)
		sl := lbl["berth.slug"]
		if sl == "" {
			continue
		}
		s := bySlug[sl]
		if s == nil {
			s = &stack.Stack{Slug: sl, ProjectPath: lbl["berth.project.path"]}
			bySlug[sl] = s
		}
		s.Containers = append(s.Containers, p.Names)
		if p.State == "running" {
			s.Running = true
		}
		if u := lbl["berth.url"]; u != "" {
			s.URL = u
		}
		if tp := lbl["berth.tilt.port"]; tp != "" {
			s.TiltPort, _ = strconv.Atoi(tp)
		}
	}
	out2 := make([]stack.Stack, 0, len(bySlug))
	for _, s := range bySlug {
		out2 = append(out2, *s)
	}
	return out2
}

// List returns all berth-managed stacks (excludes the proxy).
func List() ([]stack.Stack, error) {
	cmd := exec.Command("docker", "ps", "-a",
		"--filter", "label=berth.managed=true",
		"--filter", "label=berth.slug",
		"--format", "{{json .}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parsePS(out), nil
}

// SlugOwner returns the project path that currently owns slug, if any.
func SlugOwner(slug string) (string, bool) {
	stacks, err := List()
	if err != nil {
		return "", false
	}
	for _, s := range stacks {
		if s.Slug == slug && s.ProjectPath != "" {
			return s.ProjectPath, true
		}
	}
	return "", false
}
