// Package dockerx queries Docker for lane-managed containers.
package dockerx

import (
	"encoding/json"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/dheeraj-nalapat/lane/internal/stack"
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
		sl := lbl["lane.slug"]
		if sl == "" {
			continue
		}
		s := bySlug[sl]
		if s == nil {
			s = &stack.Stack{Slug: sl, Project: lbl["lane.project"], ProjectPath: lbl["lane.project.path"]}
			bySlug[sl] = s
		}
		s.Containers = append(s.Containers, p.Names)
		if p.State == "running" {
			s.Running = true
		}
		if u := lbl["lane.url"]; u != "" {
			s.URL = u
		}
		if tp := lbl["lane.tilt.port"]; tp != "" {
			s.TiltPort, _ = strconv.Atoi(tp)
		}
	}
	out2 := make([]stack.Stack, 0, len(bySlug))
	for _, s := range bySlug {
		out2 = append(out2, *s)
	}
	return out2
}

// List returns all lane-managed stacks (excludes the proxy).
func List() ([]stack.Stack, error) {
	cmd := exec.Command("docker", "ps", "-a",
		"--filter", "label=lane.managed=true",
		"--filter", "label=lane.slug",
		"--format", "{{json .}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parsePS(out), nil
}

// RunningServices returns the set of compose service names currently running
// for the given lane slug (compose project name).
func RunningServices(slug string) (map[string]bool, error) {
	cmd := exec.Command("docker", "ps",
		"--filter", "label=com.docker.compose.project="+slug,
		"--format", "{{json .}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseRunningServices(out), nil
}

func parseRunningServices(out []byte) map[string]bool {
	running := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var p psLine
		if json.Unmarshal([]byte(line), &p) != nil {
			continue
		}
		if p.State != "running" {
			continue
		}
		if svc := labelMap(p.Labels)["com.docker.compose.service"]; svc != "" {
			running[svc] = true
		}
	}
	return running
}

// Container is one running compose container.
type Container struct {
	Name    string
	Service string
}

// RunningContainers returns the running containers for a compose project (slug),
// each with its compose service name.
func RunningContainers(slug string) ([]Container, error) {
	cmd := exec.Command("docker", "ps",
		"--filter", "label=com.docker.compose.project="+slug,
		"--format", "{{json .}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseContainers(out), nil
}

func parseContainers(out []byte) []Container {
	var cs []Container
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		var p psLine
		if json.Unmarshal([]byte(line), &p) != nil {
			continue
		}
		if p.State != "running" {
			continue
		}
		if svc := labelMap(p.Labels)["com.docker.compose.service"]; svc != "" {
			cs = append(cs, Container{Name: p.Names, Service: svc})
		}
	}
	return cs
}

func connectArgs(network, container, alias string) []string {
	return []string{"network", "connect", "--alias", alias, network, container}
}

// NetworkConnect attaches container to network with the given DNS alias.
func NetworkConnect(network, container, alias string) error {
	return exec.Command("docker", connectArgs(network, container, alias)...).Run()
}

func disconnectArgs(network, container string) []string {
	return []string{"network", "disconnect", network, container}
}

// NetworkDisconnect detaches container from network.
func NetworkDisconnect(network, container string) error {
	return exec.Command("docker", disconnectArgs(network, container)...).Run()
}

// ForeignContainers returns the names of containers attached to network whose
// names don't belong to ownSlug's compose project (i.e. borrowed base
// containers). Compose names containers "<project>-<service>-<n>".
func ForeignContainers(network, ownSlug string) ([]string, error) {
	out, err := exec.Command("docker", "network", "inspect", network,
		"--format", "{{json .Containers}}").Output()
	if err != nil {
		return nil, err
	}
	return parseForeignContainers(out, ownSlug), nil
}

func parseForeignContainers(out []byte, ownSlug string) []string {
	var m map[string]struct {
		Name string `json:"Name"`
	}
	if json.Unmarshal(out, &m) != nil {
		return nil
	}
	prefix := ownSlug + "-"
	var foreign []string
	for _, c := range m {
		if !strings.HasPrefix(c.Name, prefix) {
			foreign = append(foreign, c.Name)
		}
	}
	sort.Strings(foreign)
	return foreign
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
