# Selective / Minimal Bring-Up + Per-Service Reach — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a developer or agent bring up a subset of a project's services (by name and/or compose profile, with dependencies auto-included) and reach every HTTP service individually at `<slug>-<service>.localhost`, reporting/waiting only on services that are actually running.

**Architecture:** lane already generates a non-invasive compose override that adds Traefik labels to *routed* services and brings the stack up via a compose or tilt runner. We extend the route set to include an auto-route per HTTP service (discovered from the compose file), pass a service/profile selection through to the runner, and make the reporting paths (`--json`, `--wait`, `ls`, `view`) consult Docker for what is actually running. Traefik only routes running containers, so labeling unselected services is harmless and we never re-implement compose's dependency/profile expansion.

**Tech Stack:** Go 1.22, cobra, `gopkg.in/yaml.v3`, Docker Compose v2, Tilt, Traefik.

**Spec:** `docs/superpowers/specs/2026-06-10-selective-service-bring-up-design.md`

**Conventions in this repo (follow them):**
- Tests are table-driven `_test.go` files alongside the source; run with `go test ./...`.
- Build everything with `go build ./...`; the binary is `go build -o lane .`.
- Each task ends with a commit. Author is configured already; do **not** add `Co-Authored-By` trailers.
- Run `go vet ./...` and `gofmt -l .` (must print nothing) before each commit.

---

## File map

| File | Responsibility | Change |
|---|---|---|
| `internal/manifest/manifest.go` | parse `.lane.toml` | routes optional; add `[autoroute]` |
| `internal/compose/compose.go` | read compose structure | add `Services()` + port discovery |
| `internal/routing/routing.go` | **new** — merge explicit + auto routes | new package |
| `internal/dockerx/dockerx.go` | query Docker | add `RunningServices()` |
| `internal/runner/runner.go` | `RunSpec` | add `Services`, `Profiles` |
| `internal/runner/compose.go` | compose invocation | profiles + service args |
| `internal/runner/tilt.go` | tilt invocation | service args, `COMPOSE_PROFILES` env |
| `internal/tiltx/tiltx.go` | tilt arg builder | accept resources |
| `cmd/root.go` | global flags | add `-C/--path`; `projectDir()` |
| `cmd/up.go` | `lane up` | positional services, `--profile`, merged routes, running-aware wait/json |
| `cmd/down.go`, `cmd/logs.go` | use `projectDir()` | drop `[path]` positional |
| `cmd/ls.go`, `cmd/view.go` | render | per-route running status (optional polish) |
| `docs/guide/selecting-services.md` | **new** docs page | usage + Tilt note |

`internal/override/override.go` needs **no change** — it already consumes a `[]Route` and labels each routed service. `cmd/restart.go` and `cmd/open.go` need no change (restart delegates to `runUp`/`runDown`; open resolves by slug, not path).

---

## Task 1: Manifest — routes optional + `[autoroute]`

**Files:**
- Modify: `internal/manifest/manifest.go`
- Test: `internal/manifest/manifest_test.go`

- [ ] **Step 1: Update the existing "no routes" test to expect success**

In `internal/manifest/manifest_test.go`, replace `TestLoad_NoRoutes` (lines 58–64) with:

```go
func TestLoad_NoRoutesNowAllowed(t *testing.T) {
	p := write(t, `name = "x"
compose_file = "docker-compose.yml"`)
	m, err := Load(p)
	if err != nil {
		t.Fatalf("routes are optional now; got error: %v", err)
	}
	if len(m.Routes) != 0 {
		t.Fatalf("got %d routes, want 0", len(m.Routes))
	}
	if !m.AutorouteEnabled() {
		t.Fatal("autoroute should default to enabled")
	}
}

func TestLoad_AutorouteBlock(t *testing.T) {
	p := write(t, `name = "x"
compose_file = "docker-compose.yml"

[autoroute]
enabled = false
exclude = ["worker", "cron"]`)
	m, err := Load(p)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if m.AutorouteEnabled() {
		t.Fatal("autoroute should be disabled")
	}
	if len(m.Autoroute.Exclude) != 2 || m.Autoroute.Exclude[0] != "worker" {
		t.Fatalf("Exclude = %v", m.Autoroute.Exclude)
	}
}
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./internal/manifest/`
Expected: FAIL — `AutorouteEnabled` undefined and `TestLoad_NoRoutesNowAllowed` errors (routes still required).

- [ ] **Step 3: Implement the manifest changes**

In `internal/manifest/manifest.go`, add the `Autoroute` type and field, the helper, and delete the zero-routes error. Replace the file body from the `Route`/`Manifest` types through `Load` with:

```go
// Route declares one web entrypoint to route through Traefik.
type Route struct {
	Service string `toml:"service"` // compose service name
	Port    int    `toml:"port"`    // internal container port
	Host    string `toml:"host"`    // optional host template, default "{slug}"
}

// Autoroute configures per-service auto-routing (every HTTP service gets a
// <slug>-<service>.localhost host unless disabled or excluded).
type Autoroute struct {
	Enabled *bool    `toml:"enabled"` // nil => default true
	Exclude []string `toml:"exclude"` // services never auto-routed
}

// Manifest is the parsed .lane.toml.
type Manifest struct {
	Name        string    `toml:"name"`
	ComposeFile string    `toml:"compose_file"`
	APITarget   string    `toml:"api_target"`
	Runner      string    `toml:"runner"`
	Routes      []Route   `toml:"routes"`
	Autoroute   Autoroute `toml:"autoroute"`
}

// AutorouteEnabled reports whether auto-routing is on (default true).
func (m *Manifest) AutorouteEnabled() bool {
	return m.Autoroute.Enabled == nil || *m.Autoroute.Enabled
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
```

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./internal/manifest/`
Expected: PASS (all manifest tests, including the two new ones).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/manifest/manifest.go internal/manifest/manifest_test.go
go vet ./internal/manifest/
git add internal/manifest/
git commit -m "feat(manifest): make routes optional; add [autoroute] config"
```

---

## Task 2: Compose — `Services()` + port discovery

**Files:**
- Modify: `internal/compose/compose.go`
- Test: `internal/compose/compose_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/compose/compose_test.go`:

```go
func TestServices_PortDiscovery(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "docker-compose.yml")
	body := `services:
  exposed:
    image: a
    expose: [8000]
  shortport:
    image: b
    ports: ["3000:80"]
  longport:
    image: c
    ports:
      - target: 9000
        published: 9001
  multi:
    image: d
    expose: [80, 443]
  none:
    image: e
  built:
    build: .
    expose: ["5000"]
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Services(p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	byName := map[string]Service{}
	for _, s := range got {
		byName[s.Name] = s
	}
	cases := map[string]int{
		"exposed":   8000, // single expose
		"shortport": 80,   // container side of short syntax
		"longport":  9000, // target of long syntax
		"multi":     0,    // ambiguous -> 0
		"none":      0,    // nothing -> 0
		"built":     5000,
	}
	for name, wantPort := range cases {
		if byName[name].Port != wantPort {
			t.Errorf("%s: Port = %d, want %d", name, byName[name].Port, wantPort)
		}
	}
	if !byName["built"].Build {
		t.Error("built: Build = false, want true")
	}
	if byName["exposed"].Build {
		t.Error("exposed: Build = true, want false")
	}
}
```

- [ ] **Step 2: Run the test, verify it fails**

Run: `go test ./internal/compose/`
Expected: FAIL — `Services` and `Service` undefined.

- [ ] **Step 3: Implement `Services()` + helpers**

Rewrite `internal/compose/compose.go` to extend the parsed struct and add discovery. Replace the whole file with:

```go
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
```

- [ ] **Step 4: Run the tests, verify they pass**

Run: `go test ./internal/compose/`
Expected: PASS (`TestServiceNames`, `TestBuiltServices`, `TestServices_PortDiscovery`).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/compose/compose.go internal/compose/compose_test.go
go vet ./internal/compose/
git add internal/compose/
git commit -m "feat(compose): add Services() with container-port discovery"
```

---

## Task 3: Routing — merge explicit + auto routes

**Files:**
- Create: `internal/routing/routing.go`
- Test: `internal/routing/routing_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/routing/routing_test.go`:

```go
package routing

import (
	"testing"

	"github.com/dheeraj-nalapat/lane/internal/compose"
	"github.com/dheeraj-nalapat/lane/internal/override"
)

func TestResolve(t *testing.T) {
	services := []compose.Service{
		{Name: "web", Port: 80},
		{Name: "api", Port: 8000},
		{Name: "admin", Port: 9000},
		{Name: "worker", Port: 0},  // no port -> skipped
		{Name: "cron", Port: 7000}, // excluded
	}
	explicit := []override.Route{
		{Service: "web", Port: 80, Hostname: "webapp.localhost"}, // explicit wins
	}
	routes, skipped := Resolve("webapp", services, explicit, true, []string{"cron"})

	byService := map[string]override.Route{}
	for _, r := range routes {
		byService[r.Service] = r
	}
	if byService["web"].Hostname != "webapp.localhost" {
		t.Errorf("web should keep explicit host, got %q", byService["web"].Hostname)
	}
	if byService["api"].Hostname != "webapp-api.localhost" {
		t.Errorf("api auto host = %q", byService["api"].Hostname)
	}
	if byService["admin"].Hostname != "webapp-admin.localhost" {
		t.Errorf("admin auto host = %q", byService["admin"].Hostname)
	}
	if _, ok := byService["cron"]; ok {
		t.Error("cron is excluded; must not be routed")
	}
	if _, ok := byService["worker"]; ok {
		t.Error("worker has no port; must not be routed")
	}
	if len(skipped) != 1 || skipped[0] != "worker" {
		t.Errorf("skipped = %v, want [worker]", skipped)
	}
}

func TestResolve_AutorouteDisabled(t *testing.T) {
	services := []compose.Service{{Name: "api", Port: 8000}}
	explicit := []override.Route{{Service: "web", Port: 80, Hostname: "webapp.localhost"}}
	routes, skipped := Resolve("webapp", services, explicit, false, nil)
	if len(routes) != 1 || routes[0].Service != "web" {
		t.Fatalf("disabled autoroute should yield only explicit routes, got %v", routes)
	}
	if len(skipped) != 0 {
		t.Fatalf("no skip reporting when autoroute disabled, got %v", skipped)
	}
}
```

- [ ] **Step 2: Run the test, verify it fails**

Run: `go test ./internal/routing/`
Expected: FAIL — package `routing` does not exist.

- [ ] **Step 3: Implement `Resolve`**

Create `internal/routing/routing.go`:

```go
// Package routing merges explicit .lane.toml routes with auto-routes derived
// from the compose file (one host per HTTP service).
package routing

import (
	"sort"

	"github.com/dheeraj-nalapat/lane/internal/compose"
	"github.com/dheeraj-nalapat/lane/internal/override"
)

// Resolve returns the merged route set and the names of services that were
// eligible for auto-routing but had no single exposed port (skipped). Explicit
// routes always win; auto-routes use <slug>-<service>.localhost. When autoroute
// is false, only explicit routes are returned and skipped is empty.
func Resolve(slug string, services []compose.Service, explicit []override.Route, autoroute bool, exclude []string) (routes []override.Route, skipped []string) {
	routed := map[string]bool{}
	for _, r := range explicit {
		routed[r.Service] = true
		routes = append(routes, r)
	}
	if !autoroute {
		return routes, nil
	}
	ex := map[string]bool{}
	for _, e := range exclude {
		ex[e] = true
	}
	// Stable order so output (and any logging) is deterministic.
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	for _, s := range services {
		if routed[s.Name] || ex[s.Name] {
			continue
		}
		if s.Port == 0 {
			skipped = append(skipped, s.Name)
			continue
		}
		routes = append(routes, override.Route{
			Service:  s.Name,
			Port:     s.Port,
			Hostname: slug + "-" + s.Name + ".localhost",
		})
	}
	return routes, skipped
}
```

- [ ] **Step 4: Run the test, verify it passes**

Run: `go test ./internal/routing/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/routing/
go vet ./internal/routing/
git add internal/routing/
git commit -m "feat(routing): merge explicit and auto per-service routes"
```

---

## Task 4: dockerx — `RunningServices`

**Files:**
- Modify: `internal/dockerx/dockerx.go`
- Test: `internal/dockerx/dockerx_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/dockerx/dockerx_test.go`:

```go
func TestParseRunningServices(t *testing.T) {
	// Two running containers + one exited, all in project "webapp".
	out := []byte(`{"Labels":"com.docker.compose.project=webapp,com.docker.compose.service=api","State":"running"}
{"Labels":"com.docker.compose.service=web,com.docker.compose.project=webapp","State":"running"}
{"Labels":"com.docker.compose.project=webapp,com.docker.compose.service=db","State":"exited"}
`)
	got := parseRunningServices(out)
	if !got["api"] || !got["web"] {
		t.Fatalf("api and web should be running: %v", got)
	}
	if got["db"] {
		t.Fatalf("db is exited; must not be running: %v", got)
	}
}
```

- [ ] **Step 2: Run the test, verify it fails**

Run: `go test ./internal/dockerx/`
Expected: FAIL — `parseRunningServices` undefined.

- [ ] **Step 3: Implement `RunningServices` + parser**

Append to `internal/dockerx/dockerx.go` (the `psLine` struct already has `Labels` and `State`):

```go
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
```

- [ ] **Step 4: Run the test, verify it passes**

Run: `go test ./internal/dockerx/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/dockerx/
go vet ./internal/dockerx/
git add internal/dockerx/
git commit -m "feat(dockerx): add RunningServices() for a compose project"
```

---

## Task 5: Runner — selection plumbing (compose + tilt)

**Files:**
- Modify: `internal/runner/runner.go`, `internal/runner/compose.go`, `internal/runner/tilt.go`, `internal/tiltx/tiltx.go`
- Test: `internal/runner/compose_test.go`, `internal/tiltx/tiltx_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/runner/compose_test.go`:

```go
func TestBuildComposeArgs_ProfilesAndServices(t *testing.T) {
	got := buildComposeArgs("webapp", "compose.yml", "ovr.yml", true,
		[]string{"minimal", "debug"}, []string{"api", "web"})
	want := []string{
		"compose", "--profile", "minimal", "--profile", "debug",
		"-p", "webapp", "-f", "compose.yml", "-f", "ovr.yml",
		"up", "-d", "--build", "api", "web",
	}
	if len(got) != len(want) {
		t.Fatalf("len mismatch:\n got %v\nwant %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg %d = %q, want %q\nfull: %v", i, got[i], want[i], got)
		}
	}
}

func TestBuildComposeArgs_NoSelection(t *testing.T) {
	got := buildComposeArgs("webapp", "compose.yml", "ovr.yml", false, nil, nil)
	want := []string{"compose", "-p", "webapp", "-f", "compose.yml", "-f", "ovr.yml", "up", "-d"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg %d = %q, want %q", i, got[i], want[i])
		}
	}
}
```

Append to `internal/tiltx/tiltx_test.go`:

```go
func TestUpArgs_Resources(t *testing.T) {
	got := UpArgs(10377, []string{"api", "web"})
	want := []string{"up", "--host", "0.0.0.0", "--port", "10377", "api", "web", "--", "--docker"}
	if len(got) != len(want) {
		t.Fatalf("len mismatch:\n got %v\nwant %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg %d = %q, want %q\nfull: %v", i, got[i], want[i], got)
		}
	}
}

func TestUpArgs_NoResources(t *testing.T) {
	got := UpArgs(10377, nil)
	want := []string{"up", "--host", "0.0.0.0", "--port", "10377", "--", "--docker"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg %d = %q, want %q", i, got[i], want[i])
		}
	}
}
```

- [ ] **Step 2: Run the tests, verify they fail**

Run: `go test ./internal/runner/ ./internal/tiltx/`
Expected: FAIL — `buildComposeArgs` takes 4 args (not 6); `UpArgs` takes 1 arg (not 2).

- [ ] **Step 3: Add `Services`/`Profiles` to `RunSpec`**

In `internal/runner/runner.go`, inside the `RunSpec` struct, add two fields after `Routes`:

```go
	Routes       []override.Route
	Services     []string // subset to bring up; empty = all
	Profiles     []string // compose profiles to activate
```

- [ ] **Step 4: Update the compose runner**

In `internal/runner/compose.go`, replace `buildComposeArgs` and its callers:

```go
// buildComposeArgs builds the `docker <args>` for bringing a stack up detached.
// Global flags (--profile, -p, -f) must precede `up`; service names follow it.
func buildComposeArgs(slug, composePath, overridePath string, build bool, profiles, services []string) []string {
	args := []string{"compose"}
	for _, p := range profiles {
		args = append(args, "--profile", p)
	}
	args = append(args, "-p", slug, "-f", composePath, "-f", overridePath, "up", "-d")
	if build {
		args = append(args, "--build")
	}
	args = append(args, services...)
	return args
}

func (composeRunner) DryRunLines(s RunSpec) string {
	return fmt.Sprintf("# runner: compose\n# command: docker %v\n",
		buildComposeArgs(s.Slug, s.ComposePath, s.OverridePath, s.Build, s.Profiles, s.Services))
}

func (composeRunner) Up(s RunSpec) error {
	printURLs(s)
	c := exec.Command("docker", buildComposeArgs(s.Slug, s.ComposePath, s.OverridePath, s.Build, s.Profiles, s.Services)...)
	c.Dir = s.Dir
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return err
	}
	emit(s, "up (detached). logs: lane logs --slug %s\n", s.Slug)
	return nil
}
```

- [ ] **Step 5: Update `tiltx.UpArgs` to accept resources**

In `internal/tiltx/tiltx.go`, replace `UpArgs`:

```go
// UpArgs returns the args for `tilt up` in lane's docker mode on a given port.
// Tilt's own flags (--host, --port) and any resource names must come BEFORE the
// `--` separator; everything after `--` is passed to the Tiltfile's config.parse
// (here just --docker). --host 0.0.0.0 is required so the Tilt UI is reachable
// from the Traefik container via host.docker.internal.
func UpArgs(port int, resources []string) []string {
	args := []string{"up", "--host", "0.0.0.0", "--port", strconv.Itoa(port)}
	args = append(args, resources...)
	return append(args, "--", "--docker")
}
```

- [ ] **Step 6: Update the tilt runner callers + profiles env**

In `internal/runner/tilt.go`, update the two `tiltx.UpArgs(s.TiltPort)` calls to pass services, and inject `COMPOSE_PROFILES`:

In `DryRunLines`, change the `tiltx.UpArgs(s.TiltPort)` call to `tiltx.UpArgs(s.TiltPort, s.Services)`.

In `Up`, change `exec.Command("tilt", tiltx.UpArgs(s.TiltPort)...)` to `tiltx.UpArgs(s.TiltPort, s.Services)`, and after `tcmd.Env = s.Env` add:

```go
	tcmd.Env = s.Env
	if len(s.Profiles) > 0 {
		tcmd.Env = append(tcmd.Env, "COMPOSE_PROFILES="+strings.Join(s.Profiles, ","))
	}
```

Add `"strings"` to the imports in `internal/runner/tilt.go`.

- [ ] **Step 7: Run the tests, verify they pass + build**

Run: `go test ./internal/runner/ ./internal/tiltx/ && go build ./...`
Expected: PASS and a clean build. (`cmd/up.go` still calls these with the new zero-value fields — `Services`/`Profiles` default nil, so existing behavior is unchanged until Task 7.)

- [ ] **Step 8: Commit**

```bash
gofmt -w internal/runner/ internal/tiltx/
go vet ./internal/runner/ ./internal/tiltx/
git add internal/runner/ internal/tiltx/
git commit -m "feat(runner): thread service/profile selection to compose and tilt"
```

---

## Task 6: CLI — global `-C/--path`, positional services

**Files:**
- Modify: `cmd/root.go`, `cmd/up.go`, `cmd/down.go`, `cmd/logs.go`
- Test: `cmd/root_test.go`

This task is mechanical wiring; the route/selection logic lands in Task 7. After this task the binary still behaves as before except that the project directory comes from `-C` instead of a positional arg.

- [ ] **Step 1: Add the global flag + new `projectDir()` to root**

In `cmd/root.go`, add `flagPath` to the var block and register the persistent flag in `init`:

```go
var (
	flagSlug    string
	flagDryRun  bool
	flagVerbose bool
	flagPath    string
)
```

In `init`, add:

```go
	root.PersistentFlags().StringVarP(&flagPath, "path", "C", "", "project directory (default: current directory)")
```

- [ ] **Step 2: Move `projectDir` to take no args**

In `cmd/up.go`, replace the `projectDir` function (lines 225–230) with:

```go
func projectDir() (string, error) {
	if flagPath != "" {
		return filepath.Abs(flagPath)
	}
	return os.Getwd()
}
```

- [ ] **Step 3: Update the three callers + command signatures**

In `cmd/up.go`: change `dir, err := projectDir(args)` to `dir, err := projectDir()`; change the command's `Use` to `"up [services...]"` and `Args` to `cobra.ArbitraryArgs`.

In `cmd/down.go`: change `dir, err := projectDir(args)` to `dir, err := projectDir()`; change `Use` to `"down"` and `Args` to `cobra.NoArgs`.

In `cmd/logs.go`: change `dir, err := projectDir(args)` to `dir, err := projectDir()`; change `Use` to `"logs"` and `Args` to `cobra.NoArgs`.

(`cmd/restart.go` keeps delegating `runDown(cmd, args)` / `runUp(cmd, args)`; the `args` it forwards are now service names, which `runUp` will use and `runDown` ignores. Update its `Use` to `"restart [services...]"` and `Args` to `cobra.ArbitraryArgs`.)

- [ ] **Step 4: Add a smoke test for the flag wiring**

Append to `cmd/root_test.go`:

```go
func TestPathFlagRegistered(t *testing.T) {
	if root.PersistentFlags().Lookup("path") == nil {
		t.Fatal("--path persistent flag not registered")
	}
	if root.PersistentFlags().ShorthandLookup("C") == nil {
		t.Fatal("-C shorthand not registered")
	}
}
```

- [ ] **Step 5: Build + test**

Run: `go build ./... && go test ./cmd/`
Expected: clean build; PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -w cmd/
go vet ./cmd/
git add cmd/
git commit -m "feat(cli): add global -C/--path; up/restart take positional service names"
```

---

## Task 7: Wire selection + auto-routing into `lane up`

**Files:**
- Modify: `cmd/up.go`

- [ ] **Step 1: Add the `--profile` flag and capture services**

In `cmd/up.go`, add to the `var (...)` block:

```go
	flagProfiles []string
```

In `init()`, add:

```go
	upCmd.Flags().StringSliceVarP(&flagProfiles, "profile", "p", nil, "compose profile(s) to activate (repeatable)")
```

- [ ] **Step 2: Build the merged route set from compose + manifest**

In `runUp`, replace the explicit-routes block (currently lines 123–129, building `routes` from `m.Routes` only) with the following. It renders explicit routes, reads compose services, and merges via `routing.Resolve`:

```go
	var explicit []override.Route
	for _, r := range m.Routes {
		explicit = append(explicit, override.Route{
			Service: r.Service, Port: r.Port,
			Hostname: identity.RenderHost(r.Host, sl),
		})
	}
	composePath := filepath.Join(dir, m.ComposeFile)
	svcInfos, err := compose.Services(composePath)
	if err != nil {
		return err
	}
	routes, skipped := routing.Resolve(sl, svcInfos, explicit, m.AutorouteEnabled(), m.Autoroute.Exclude)
	if len(skipped) > 0 && !flagJSON {
		fmt.Fprintf(os.Stderr, "lane: not auto-routed (no single exposed port): %s\n", strings.Join(skipped, ", "))
	}
```

Then **delete** the later duplicate `composePath := filepath.Join(dir, m.ComposeFile)` line (currently line 143) and the now-misplaced `svcs, err := compose.ServiceNames(composePath)` call — replace that pair (lines 143–147) with just:

```go
	svcs, err := compose.ServiceNames(composePath)
	if err != nil {
		return err
	}
```

(Keep `svcs` — it is still passed to `override.Generate` as the full service list. `composePath` is now defined earlier.)

Add imports to `cmd/up.go`: `"strings"`, and `"github.com/dheeraj-nalapat/lane/internal/routing"`. (`compose`, `override`, `identity` are already imported.)

- [ ] **Step 3: Pass selection to the runner**

In `runUp`, find the `spec := runner.RunSpec{...}` literal and add the two selection fields:

```go
	spec := runner.RunSpec{
		Slug: sl, Dir: dir, ComposePath: composePath, OverridePath: overridePath,
		Routes: routes, Detach: detach, Build: flagBuild,
		TiltPort: tiltPort, DynamicPath: dynamicPath, Env: env, TLS: tlsOn,
		Quiet:    flagJSON,
		Services: args,          // positional service names (subset)
		Profiles: flagProfiles,
	}
```

- [ ] **Step 4: Make `--wait` and `--json` running-aware**

Add a `Running` field to `upURL` (struct near the top of `cmd/up.go`):

```go
type upURL struct {
	Service string `json:"service"`
	Host    string `json:"host"`
	URL     string `json:"url"`
	Running bool   `json:"running"`
}
```

Change `buildUpResult` to accept and apply a running set:

```go
func buildUpResult(slug, runnerName string, tlsOn bool, routes []override.Route, tiltPort int, running map[string]bool) upResult {
	res := upResult{Slug: slug, Runner: runnerName, TLS: tlsOn}
	for _, r := range routes {
		res.URLs = append(res.URLs, upURL{
			Service: r.Service, Host: r.Hostname, URL: "http://" + r.Hostname,
			Running: running[r.Service],
		})
	}
	if tiltPort > 0 {
		res.TiltURL = "http://tilt-" + slug + ".localhost"
	}
	return res
}
```

Add a helper that returns only the URLs of running routes (for `--wait`):

```go
func runningRouteURLs(routes []override.Route, running map[string]bool) []string {
	var u []string
	for _, r := range routes {
		if running[r.Service] {
			u = append(u, "http://"+r.Hostname)
		}
	}
	return u
}
```

Update the "already running" early-return branch (the `if claimed, ok := dockerx.SlugOwner(sl); ok {` block) so its `buildUpResult` call passes a running set:

```go
		if claimed == dir {
			if flagJSON {
				running, _ := dockerx.RunningServices(sl)
				return printJSON(buildUpResult(sl, runnerName, tlsx.Enabled(), routes, 0, running))
			}
			fmt.Printf("stack %q already running — use `lane restart` to recreate, or `lane down` to stop\n", sl)
			return nil
		}
```

(Note: in that early branch `routes` must already be computed. Move the route-building block from Step 2 to **above** the `dockerx.SlugOwner` check so `routes` is in scope there. The compose-services read has no side effects, so this reordering is safe.)

Replace the tail of `runUp` (the `if flagWait {...}` and `if flagJSON {...}` blocks) with:

```go
	if flagWait || flagJSON {
		running, err := dockerx.RunningServices(sl)
		if err != nil {
			return err
		}
		if flagWait {
			if err := ready.WaitReady(runningRouteURLs(routes, running), flagWaitTimeout, nil); err != nil {
				return err
			}
		}
		if flagJSON {
			return printJSON(buildUpResult(sl, runnerName, tlsOn, routes, tiltPort, running))
		}
	}
	return nil
```

Delete the now-unused `routeURLs` function (lines 59–65) — it is replaced by `runningRouteURLs`.

- [ ] **Step 5: Build + run all tests**

Run: `go build ./... && go test ./...`
Expected: clean build; all packages PASS.

- [ ] **Step 6: Manual smoke check (dry-run, no Docker needed)**

Run:
```bash
go build -o /tmp/lane .
cd <a project with .lane.toml + compose>   # or use -C
/tmp/lane up api --profile minimal --dry-run -C <project>
```
Expected: the printed compose command contains `--profile minimal ... up -d ... api`, and the override block lists Traefik routers including `<slug>-<service>` hosts for auto-routed services.

- [ ] **Step 7: Commit**

```bash
gofmt -w cmd/up.go
go vet ./...
git add cmd/up.go
git commit -m "feat(up): subset selection, auto-routing, running-aware wait/json"
```

---

## Task 8: Show running status in `ls` / `view` (polish)

**Files:**
- Modify: `cmd/ls.go` (and `cmd/view.go` / `internal/ui` if they render per-route URLs)

This is a presentation-only task. The `Stack` model (`internal/stack/stack.go`) already has a `Running` bool per stack. Per-service running detail is available via `dockerx.RunningServices(slug)`.

- [ ] **Step 1: Read the current renderers**

Open `cmd/ls.go` and `cmd/view.go`. Identify where each stack's URL(s) are printed.

- [ ] **Step 2: Annotate per-route status where routes are listed**

Wherever a route URL is printed for a stack, call `dockerx.RunningServices(s.Slug)` once per stack and suffix non-running routes with ` (not running)`. Concretely, in the loop that prints a stack's routes, add before the inner loop:

```go
running, _ := dockerx.RunningServices(s.Slug)
```

and when printing each route URL, append the marker:

```go
status := ""
if !running[route.Service] {
	status = "  (not running)"
}
fmt.Printf("  → %s%s\n", url, status)
```

(Adapt variable names to the actual loop. If `ls`/`view` currently print only the single `stack.URL` label and not per-service routes, leave them unchanged and instead ensure `lane up --json` — already running-aware from Task 7 — is the machine-facing source of per-service status. In that case, mark this task done with a one-line note in the commit message.)

- [ ] **Step 3: Build + test**

Run: `go build ./... && go test ./...`
Expected: clean build; PASS.

- [ ] **Step 4: Commit**

```bash
gofmt -w cmd/
go vet ./...
git add cmd/
git commit -m "feat(ls,view): mark not-running services in route listings"
```

---

## Task 9: Docs — "Selecting & reaching services"

**Files:**
- Create: `docs/guide/selecting-services.md`
- Modify: `mkdocs.yml` (add to nav)

- [ ] **Step 1: Write the guide page**

Create `docs/guide/selecting-services.md`:

```markdown
# Selecting & reaching services

By default `lane up` brings up your whole stack. You can bring up just part of it.

## Bring up a subset

Pass service names (dependencies come up automatically):

    lane up api            # api + whatever it depends_on
    lane up web api

Or activate a Docker Compose profile (defined in your compose file):

    lane up --profile minimal
    lane up api --profile debug

`lane up` with no arguments still brings up everything.

## Reaching each service

Every HTTP service is reachable at a dashed host derived from the slug:

    <slug>-<service>.localhost

So in the `webapp` stack, `api` is at `http://webapp-api.localhost` and `admin`
at `http://webapp-admin.localhost` — no configuration needed. An explicit
`[[routes]]` entry overrides the host for a service (e.g. the bare
`webapp.localhost`).

A service is auto-routed when lane can find a single container port for it (from
`expose:` or the container side of `ports:`). Services with no port, or more than
one, are skipped — add a `[[routes]]` entry to route those explicitly.

Disable or trim auto-routing in `.lane.toml`:

    [autoroute]
    enabled = true            # default
    exclude = ["worker"]      # never auto-route these

## For agents

`lane up --wait --json` returns the per-service URLs and whether each is running,
and waits only on the services you actually started:

    lane up api --wait --json

## Tilt note

With the compose runner, profiles and service selection work directly. Under the
Tilt runner, selected service names map to Tilt resources; compose profiles are
passed via the `COMPOSE_PROFILES` environment variable, which your Tiltfile shim
forwards to `docker_compose(..., profiles=...)`. If your Tiltfile does not forward
profiles, use service-name selection (works on both runners) or the compose runner.
```

- [ ] **Step 2: Add to nav**

In `mkdocs.yml`, add `- Selecting services: selecting-services.md` to the `nav:` list (after the getting-started/recipes entries — match the existing nav style).

- [ ] **Step 3: Build the docs strictly**

Run:
```bash
. .venv-docs/bin/activate && mkdocs build --strict
```
Expected: builds with no warnings/errors.

- [ ] **Step 4: Commit**

```bash
git add docs/guide/selecting-services.md mkdocs.yml
git commit -m "docs: add Selecting & reaching services guide"
```

---

## Task 10: End-to-end verification

**Files:** none (manual/scripted verification against real Docker).

This mirrors the existing whoami-style live check used earlier in the project. It needs Docker running and must not disturb other stacks.

- [ ] **Step 1: Create a throwaway multi-service project**

```bash
mkdir -p /tmp/lane-e2e && cd /tmp/lane-e2e
cat > docker-compose.yml <<'YAML'
services:
  web:
    image: traefik/whoami
    expose: ["80"]
  api:
    image: traefik/whoami
    command: ["--port", "8000"]
    expose: ["8000"]
  worker:
    image: alpine
    command: ["sleep", "infinity"]
YAML
cat > .lane.toml <<'TOML'
name = "e2e"
compose_file = "docker-compose.yml"
TOML
```

- [ ] **Step 2: Bring up only `web`, with deps, wait + json**

```bash
/tmp/lane up web --wait --json -C /tmp/lane-e2e
```
Expected JSON: `web` route present with `"running": true`; `api` route present with `"running": false`; `worker` not in routes; stderr noted `worker` as not auto-routed.

- [ ] **Step 3: Confirm reachability and isolation**

```bash
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: e2e-web.localhost' http://127.0.0.1   # 200
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: e2e-api.localhost' http://127.0.0.1   # 502/404 (api not started)
```
Expected: `e2e-web.localhost` → 200; `e2e-api.localhost` → not 200 (api wasn't selected).

- [ ] **Step 4: Bring up `api` too, confirm it becomes reachable**

```bash
/tmp/lane up api --wait --json -C /tmp/lane-e2e
curl -s -o /dev/null -w '%{http_code}\n' -H 'Host: e2e-api.localhost' http://127.0.0.1   # 200
```

- [ ] **Step 5: Tear down**

```bash
/tmp/lane down -C /tmp/lane-e2e
rm -rf /tmp/lane-e2e
```
Expected: stack removed; no leftover containers (`docker ps --filter label=lane.slug=e2e` is empty).

- [ ] **Step 6: Commit (verification notes only, if any tracked artifacts)**

No code changes here. If you captured a verification log under `docs/`, commit it; otherwise nothing to commit.

---

## Self-review notes (author)

- **Spec coverage:** §2 CLI → Task 6/7; §3 config → Task 1; §4 runners → Task 5/7; §5 port discovery → Task 2, route merge → Task 3; §6 override → unchanged (verified consumes merged routes); §7 reporting → Task 4 + Task 7 (json/wait) + Task 8 (ls/view); §8 errors → Task 7 (skipped log) + compose's own unknown-service/profile errors; §9 testing → Tasks 1–5 unit, Task 10 e2e. No gaps.
- **Unknown-service error (§8):** compose itself errors on an unknown service name (`no such service`); lane surfaces it via the runner's non-zero exit. A pre-validation against `compose.ServiceNames` is a nice-to-have but not required for v1 — omitted to avoid duplicating compose's check. If desired, add a guard in Task 7 Step 3 comparing `args` against `svcs`.
- **Type consistency:** `compose.Service{Name,Build,Port}`, `routing.Resolve(slug, []compose.Service, []override.Route, bool, []string) ([]override.Route, []string)`, `dockerx.RunningServices(slug) (map[string]bool, error)`, `RunSpec.Services/.Profiles`, `buildComposeArgs(...,profiles,services)`, `tiltx.UpArgs(port, resources)`, `upURL.Running` — all consistent across tasks.
