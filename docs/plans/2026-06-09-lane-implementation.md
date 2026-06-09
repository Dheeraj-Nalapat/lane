# lane Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `lane`, a Go CLI that runs many Docker/Tilt project stacks at once — across projects and git worktrees — with zero host-port conflicts, each reachable at a friendly `*.localhost` hostname via a shared Traefik proxy.

**Architecture:** A shared, always-on Traefik container (on an external `lane` network) routes `Host(<slug>.localhost)` to each stack's containers over the Docker network, so stacks publish **no host ports**. `lane` derives a per-stack `slug`, generates a non-invasive Compose override (strips host ports + hardcoded container names via `!reset`, adds Traefik labels + network), sets env, and runs `tilt up -- --docker`. State is derived from Docker labels (no state file). lane is **service-agnostic** — it routes whatever the manifest names.

**Tech Stack:** Go 1.22, `spf13/cobra` (CLI), `BurntSushi/toml` (manifest), `gopkg.in/yaml.v3` (override + compose parsing), `charmbracelet/bubbletea` + `lipgloss` (the `view` TUI). External tools assumed present at runtime: Docker ≥ 28 / Compose ≥ 2.20, Tilt ≥ 0.37, git.

---

## File Structure

```
lane/
  go.mod                          module github.com/dheeraj-nalapat/lane (adjust to real repo path)
  main.go                         entrypoint → cmd.Execute()
  cmd/
    root.go                       root cobra command + global flags (--slug, --dry-run, -v)
    up.go                         lane up
    down.go                       lane down
    ls.go                         lane ls
    view.go                       lane view (+ --watch)
    proxy.go                      lane proxy up|down|status|logs
    doctor.go                     lane doctor
    init.go                       lane init
    open.go                       lane open
    logs.go                       lane logs
  internal/
    ports/ports.go                free TCP port allocation
    gitx/worktree.go              linked-worktree detection + name
    manifest/manifest.go          .lane.toml load + validate
    slug/slug.go                  sanitize + derive + resolution ladder
    compose/compose.go            read base compose → service names
    override/override.go          generate the Compose override (labels, !reset, network)
    paths/paths.go                ~/.lane dirs (home, overrides, run, traefik/dynamic)
    proxy/proxy.go                Traefik lifecycle: ensure network, up/down/status, dynamic file
    dockerx/dockerx.go            query containers by lane labels → []Stack
    traefikapi/traefikapi.go      query Traefik API (routers/services) for `view`
    tiltx/tiltx.go                build the `tilt up` command + env
    stack/stack.go                Stack struct shared across ls/view/down
  assets/
    traefik-compose.yml.tmpl      embedded Traefik compose template
  docs/...
  .goreleaser.yaml                release config
  install.sh                      curl|sh installer
```

**Conventions for every Go task:** tests live next to the code as `<file>_test.go` in the same package. Run tests with `go test ./...`. Commit after each task with the message shown.

---

## Phase 0 — Scaffold

### Task 1: Project scaffold + root command

**Files:**
- Create: `lane/go.mod`, `lane/main.go`, `lane/cmd/root.go`
- Test: `lane/cmd/root_test.go`

- [ ] **Step 1: Initialize the module**

Run from `lane/`:
```bash
go mod init github.com/dheeraj-nalapat/lane   # adjust to the real GitHub path you'll publish under
go get github.com/spf13/cobra@latest
```
Expected: `go.mod` created with a `require github.com/spf13/cobra` line.

- [ ] **Step 2: Write the failing test**

`cmd/root_test.go`:
```go
package cmd

import (
	"bytes"
	"testing"
)

func TestRootCommand_HasName(t *testing.T) {
	if root.Use != "lane" {
		t.Fatalf("root.Use = %q, want %q", root.Use, "lane")
	}
}

func TestRootCommand_HelpRuns(t *testing.T) {
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("--help produced no output")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./cmd/`
Expected: FAIL — `undefined: root`.

- [ ] **Step 4: Implement the root command**

`cmd/root.go`:
```go
package cmd

import "github.com/spf13/cobra"

var (
	flagSlug   string
	flagDryRun bool
	flagVerbose bool
)

var root = &cobra.Command{
	Use:           "lane",
	Short:         "Run many project stacks at once with zero port conflicts",
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	root.PersistentFlags().StringVar(&flagSlug, "slug", "", "override the stack slug")
	root.PersistentFlags().BoolVar(&flagDryRun, "dry-run", false, "print what would happen, then exit")
	root.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "verbose output")
}

// Execute runs the CLI.
func Execute() error { return root.Execute() }
```

`main.go`:
```go
package main

import (
	"fmt"
	"os"

	"github.com/dheeraj-nalapat/lane/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "lane:", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 5: Run tests + build**

Run: `go test ./cmd/ && go build ./...`
Expected: PASS, binary builds.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum main.go cmd/
git commit -m "feat: scaffold lane module and root command"
```

---

## Phase 1 — Pure logic (slug, ports, manifest, git)

### Task 2: Free port allocation

**Files:**
- Create: `lane/internal/ports/ports.go`
- Test: `lane/internal/ports/ports_test.go`

- [ ] **Step 1: Write the failing test**

`internal/ports/ports_test.go`:
```go
package ports

import (
	"net"
	"strconv"
	"testing"
)

func TestFree_ReturnsUsablePort(t *testing.T) {
	p, err := Free()
	if err != nil {
		t.Fatalf("Free() error: %v", err)
	}
	if p <= 0 || p > 65535 {
		t.Fatalf("Free() = %d, out of range", p)
	}
	// The port must be bindable right after Free returns.
	l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(p)))
	if err != nil {
		t.Fatalf("returned port %d not bindable: %v", p, err)
	}
	_ = l.Close()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ports/`
Expected: FAIL — `undefined: Free`.

- [ ] **Step 3: Implement**

`internal/ports/ports.go`:
```go
// Package ports allocates free TCP ports on the host.
package ports

import "net"

// Free asks the kernel for an unused TCP port by binding to :0, then releases
// it. There is an inherent TOCTOU window, but it is sufficient for picking a
// Tilt UI port at startup.
func Free() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/ports/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ports/
git commit -m "feat: free TCP port allocation"
```

---

### Task 3: Git linked-worktree detection

**Files:**
- Create: `lane/internal/gitx/worktree.go`
- Test: `lane/internal/gitx/worktree_test.go`

- [ ] **Step 1: Write the failing test**

`internal/gitx/worktree_test.go`:
```go
package gitx

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestWorktree_MainCheckoutReturnsFalse(t *testing.T) {
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	name, linked, err := Worktree(dir)
	if err != nil {
		t.Fatalf("Worktree error: %v", err)
	}
	if linked {
		t.Fatalf("main checkout reported as linked worktree (name=%q)", name)
	}
}

func TestWorktree_LinkedReturnsName(t *testing.T) {
	main := t.TempDir()
	git(t, main, "init", "-q")
	git(t, main, "-c", "user.email=a@b.c", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init")
	wt := filepath.Join(t.TempDir(), "featx")
	git(t, main, "worktree", "add", "-q", wt)

	name, linked, err := Worktree(wt)
	if err != nil {
		t.Fatalf("Worktree error: %v", err)
	}
	if !linked {
		t.Fatal("linked worktree not detected")
	}
	if name != "featx" {
		t.Fatalf("worktree name = %q, want %q", name, "featx")
	}
}

func TestWorktree_NotARepo(t *testing.T) {
	_, linked, err := Worktree(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if linked {
		t.Fatal("non-repo reported as linked worktree")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/gitx/`
Expected: FAIL — `undefined: Worktree`.

- [ ] **Step 3: Implement**

`internal/gitx/worktree.go`:
```go
// Package gitx provides git worktree helpers for slug derivation.
package gitx

import (
	"os/exec"
	"path/filepath"
	"strings"
)

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// Worktree reports whether dir is inside a *linked* git worktree, and if so its
// name (the leaf of .git/worktrees/<name>). For the main checkout, a
// non-git dir, or any error treated as "not a repo", it returns ("", false, nil).
func Worktree(dir string) (name string, linked bool, err error) {
	gitDir, err := run(dir, "rev-parse", "--absolute-git-dir")
	if err != nil {
		return "", false, nil // not a git repo
	}
	commonDir, err := run(dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", false, err
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(dir, commonDir)
	}
	gitDir = filepath.Clean(gitDir)
	commonDir = filepath.Clean(commonDir)
	if gitDir == commonDir {
		return "", false, nil // main checkout
	}
	return filepath.Base(gitDir), true, nil
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/gitx/`
Expected: PASS (all three).

- [ ] **Step 5: Commit**

```bash
git add internal/gitx/
git commit -m "feat: detect linked git worktree and its name"
```

---

### Task 4: Manifest (.lane.toml) loading

**Files:**
- Create: `lane/internal/manifest/manifest.go`
- Test: `lane/internal/manifest/manifest_test.go`

- [ ] **Step 1: Write the failing test**

`internal/manifest/manifest_test.go`:
```go
package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, ".lane.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_Valid(t *testing.T) {
	p := write(t, `
name = "remind"
compose_file = "infra/docker-compose.yml"

[[routes]]
service = "ui"
port = 80

[[routes]]
service = "server"
port = 8000
host = "api.{slug}"
`)
	m, err := Load(p)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if m.Name != "remind" {
		t.Fatalf("Name = %q", m.Name)
	}
	if m.ComposeFile != "infra/docker-compose.yml" {
		t.Fatalf("ComposeFile = %q", m.ComposeFile)
	}
	if len(m.Routes) != 2 {
		t.Fatalf("got %d routes, want 2", len(m.Routes))
	}
	if m.Routes[1].Host != "api.{slug}" {
		t.Fatalf("route[1].Host = %q", m.Routes[1].Host)
	}
}

func TestLoad_MissingName(t *testing.T) {
	p := write(t, `compose_file = "docker-compose.yml"`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestLoad_NoRoutes(t *testing.T) {
	p := write(t, `name = "x"
compose_file = "docker-compose.yml"`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for zero routes")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/manifest/`
Expected: FAIL — `undefined: Load`. (May need `go get github.com/BurntSushi/toml@latest` first.)

- [ ] **Step 3: Implement**

`internal/manifest/manifest.go`:
```go
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
	Name        string `toml:"name"`         // base slug
	ComposeFile string `toml:"compose_file"` // path to base compose, relative to project dir
	APITarget   string `toml:"api_target"`   // optional, e.g. "server:8000" for dev-server /api proxying
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

- [ ] **Step 4: Run test**

Run: `go test ./internal/manifest/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/manifest/ go.mod go.sum
git commit -m "feat: load and validate .lane.toml manifest"
```

---

### Task 5: Slug sanitize + derive + resolution ladder

**Files:**
- Create: `lane/internal/slug/slug.go`
- Test: `lane/internal/slug/slug_test.go`

- [ ] **Step 1: Write the failing test**

`internal/slug/slug_test.go`:
```go
package slug

import "testing"

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"ReMind":        "remind",
		"feature/Foo_X": "feature-foo-x",
		"--weird--":     "weird",
		"a..b__c":       "a-b-c",
	}
	for in, want := range cases {
		if got := Sanitize(in); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitize_CapsLength(t *testing.T) {
	long := ""
	for i := 0; i < 100; i++ {
		long += "a"
	}
	if got := Sanitize(long); len(got) != 40 {
		t.Fatalf("len = %d, want 40", len(got))
	}
}

func TestResolve_Ladder(t *testing.T) {
	tests := []struct {
		name string
		in   Inputs
		want string
	}{
		{"flag wins", Inputs{Flag: "Foo", Env: "bar", ManifestName: "baz", DirBase: "qux"}, "foo"},
		{"env next", Inputs{Env: "Bar", ManifestName: "baz"}, "bar"},
		{"manifest main", Inputs{ManifestName: "remind"}, "remind"},
		{"manifest worktree", Inputs{ManifestName: "remind", Worktree: "featx"}, "remind-featx"},
		{"dir fallback", Inputs{DirBase: "myproj", Worktree: "wt"}, "myproj-wt"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Resolve(tc.in); got != tc.want {
				t.Errorf("Resolve = %q, want %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/slug/`
Expected: FAIL — `undefined: Sanitize`.

- [ ] **Step 3: Implement**

`internal/slug/slug.go`:
```go
// Package slug derives DNS/Docker-safe stack identities.
package slug

import (
	"regexp"
	"strings"
)

const maxLen = 40

var (
	nonSafe   = regexp.MustCompile(`[^a-z0-9-]+`)
	multiDash = regexp.MustCompile(`-{2,}`)
)

// Sanitize lowercases s and reduces it to a safe DNS label: [a-z0-9-], no
// repeated/edge dashes, capped at 40 chars.
func Sanitize(s string) string {
	s = strings.ToLower(s)
	s = nonSafe.ReplaceAllString(s, "-")
	s = multiDash.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > maxLen {
		s = strings.Trim(s[:maxLen], "-")
	}
	return s
}

// Derive joins a base name with an optional worktree suffix, then sanitizes.
func Derive(base, worktree string) string {
	if worktree != "" {
		base = base + "-" + worktree
	}
	return Sanitize(base)
}

// Inputs feeds the resolution ladder.
type Inputs struct {
	Flag         string // --slug
	Env          string // LANE_SLUG
	ManifestName string // .lane.toml name
	Worktree     string // linked worktree name, "" if main
	DirBase      string // basename of project dir (fallback)
}

// Resolve applies the precedence ladder: flag > env > manifest(+worktree) > dir(+worktree).
func Resolve(in Inputs) string {
	switch {
	case in.Flag != "":
		return Sanitize(in.Flag)
	case in.Env != "":
		return Sanitize(in.Env)
	case in.ManifestName != "":
		return Derive(in.ManifestName, in.Worktree)
	default:
		return Derive(in.DirBase, in.Worktree)
	}
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/slug/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/slug/
git commit -m "feat: slug sanitize, derive, and resolution ladder"
```

---

## Phase 2 — Compose override generation

### Task 6: Read base compose service names

**Files:**
- Create: `lane/internal/compose/compose.go`
- Test: `lane/internal/compose/compose_test.go`

- [ ] **Step 1: Write the failing test**

`internal/compose/compose_test.go`:
```go
package compose

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestServiceNames(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "docker-compose.yml")
	body := `services:
  server:
    image: x
  ui:
    image: y
volumes:
  data:
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ServiceNames(p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	sort.Strings(got)
	want := []string{"server", "ui"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/compose/`
Expected: FAIL — `undefined: ServiceNames`. (Run `go get gopkg.in/yaml.v3@latest` first.)

- [ ] **Step 3: Implement**

`internal/compose/compose.go`:
```go
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
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/compose/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/compose/
git commit -m "feat: read service names from base compose"
```

---

### Task 7: Generate the Compose override

**Files:**
- Create: `lane/internal/override/override.go`
- Test: `lane/internal/override/override_test.go`

This generates the non-invasive overlay: for every service, `!reset` host ports and any hardcoded `container_name`; for routed services, join the `lane` network and add Traefik + lane labels.

- [ ] **Step 1: Write the failing test**

`internal/override/override_test.go`:
```go
package override

import (
	"strings"
	"testing"
)

func TestGenerate(t *testing.T) {
	out, err := Generate(Spec{
		Slug:        "remind-featx",
		ProjectPath: "/home/u/remind",
		Network:     "lane",
		Services:    []string{"server", "agent-server", "ui", "worker"},
		TiltPort:    10377,
		Routes: []Route{
			{Service: "ui", Port: 80, Hostname: "remind-featx.localhost"},
			{Service: "server", Port: 8000, Hostname: "api.remind-featx.localhost"},
		},
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}
	s := string(out)

	// Every service gets ports + container_name reset.
	for _, svc := range []string{"server:", "agent-server:", "ui:", "worker:"} {
		if !strings.Contains(s, svc) {
			t.Errorf("missing service %q in:\n%s", svc, s)
		}
	}
	if c := strings.Count(s, "ports: !reset []"); c != 4 {
		t.Errorf("got %d 'ports: !reset []', want 4\n%s", c, s)
	}
	if c := strings.Count(s, "container_name: !reset null"); c != 4 {
		t.Errorf("got %d 'container_name: !reset null', want 4", c)
	}

	// Routed services get a Traefik host rule + the service port label.
	if !strings.Contains(s, "Host(`remind-featx.localhost`)") {
		t.Errorf("missing ui host rule\n%s", s)
	}
	if !strings.Contains(s, "Host(`api.remind-featx.localhost`)") {
		t.Errorf("missing server host rule")
	}
	if !strings.Contains(s, "loadbalancer.server.port=80") {
		t.Errorf("missing ui service port label")
	}

	// lane identity labels appear.
	if !strings.Contains(s, "lane.slug=remind-featx") {
		t.Errorf("missing lane.slug label")
	}
	if !strings.Contains(s, "lane.project.path=/home/u/remind") {
		t.Errorf("missing lane.project.path label")
	}
	if !strings.Contains(s, "lane.tilt.port=10377") {
		t.Errorf("missing lane.tilt.port label")
	}

	// The external network is declared.
	if !strings.Contains(s, "external: true") {
		t.Errorf("missing external network declaration")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/override/`
Expected: FAIL — `undefined: Generate`.

- [ ] **Step 3: Implement**

`internal/override/override.go`:
```go
// Package override generates the non-invasive Compose overlay lane applies.
package override

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// Route is a resolved web entrypoint (hostname already rendered).
type Route struct {
	Service  string
	Port     int
	Hostname string
}

// Spec is the input to Generate.
type Spec struct {
	Slug        string
	ProjectPath string
	Network     string // shared external network name, e.g. "lane"
	Services    []string
	Routes      []Route
	TiltPort    int
}

// resetNode emits a Compose `!reset` override. seq=true → `!reset []`,
// otherwise `!reset null`.
type resetNode struct{ seq bool }

func (r resetNode) MarshalYAML() (any, error) {
	if r.seq {
		return &yaml.Node{Kind: yaml.SequenceNode, Tag: "!reset"}, nil
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!reset", Value: "null"}, nil
}

// Generate returns the override YAML bytes.
func Generate(s Spec) ([]byte, error) {
	routed := map[string]Route{}
	for _, r := range s.Routes {
		routed[r.Service] = r
	}

	idLabels := []string{
		"lane.managed=true",
		"lane.slug=" + s.Slug,
		"lane.project.path=" + s.ProjectPath,
		fmt.Sprintf("lane.tilt.port=%d", s.TiltPort),
	}

	services := map[string]any{}
	svcNames := append([]string(nil), s.Services...)
	sort.Strings(svcNames)
	for _, name := range svcNames {
		svc := map[string]any{
			"container_name": resetNode{},        // !reset null
			"ports":          resetNode{seq: true}, // !reset []
		}
		labels := append([]string(nil), idLabels...)
		if r, ok := routed[name]; ok {
			svc["networks"] = []string{"default", s.Network}
			router := s.Slug + "-" + name
			labels = append(labels,
				"traefik.enable=true",
				"traefik.docker.network="+s.Network,
				fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", router, r.Hostname),
				"traefik.http.routers."+router+".entrypoints=web",
				fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%d", router, r.Port),
				"lane.url=http://"+r.Hostname,
			)
		}
		svc["labels"] = labels
		services[name] = svc
	}

	doc := map[string]any{
		"services": services,
		"networks": map[string]any{
			s.Network: map[string]any{"external": true},
		},
	}
	return yaml.Marshal(doc)
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/override/`
Expected: PASS.

> **If `!reset` is not emitted verbatim** (yaml.v3 version quirk): the test will show the actual output. Fallback — after `yaml.Marshal`, replace the sentinel: emit `ports: ["__RESET__"]` / `container_name: "__RESET__"` from plain values and `bytes.ReplaceAll` them to `!reset []` / `!reset null`. Keep the test assertions unchanged.

- [ ] **Step 5: Commit**

```bash
git add internal/override/
git commit -m "feat: generate non-invasive compose override with !reset + traefik labels"
```

---

## Phase 3 — Paths and the shared proxy

### Task 8: lane home paths

**Files:**
- Create: `lane/internal/paths/paths.go`
- Test: `lane/internal/paths/paths_test.go`

- [ ] **Step 1: Write the failing test**

`internal/paths/paths_test.go`:
```go
package paths

import (
	"strings"
	"testing"
)

func TestPaths(t *testing.T) {
	t.Setenv("LANE_HOME", "/tmp/lanetest")
	if got := Home(); got != "/tmp/lanetest" {
		t.Fatalf("Home = %q", got)
	}
	if !strings.HasSuffix(Overrides(), "/overrides") {
		t.Fatalf("Overrides = %q", Overrides())
	}
	if !strings.HasSuffix(Run(), "/run") {
		t.Fatalf("Run = %q", Run())
	}
	if !strings.HasSuffix(TraefikDynamic(), "/traefik/dynamic") {
		t.Fatalf("TraefikDynamic = %q", TraefikDynamic())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/paths/`
Expected: FAIL — `undefined: Home`.

- [ ] **Step 3: Implement**

`internal/paths/paths.go`:
```go
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
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/paths/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/paths/
git commit -m "feat: lane home path layout"
```

---

### Task 9: Traefik compose asset + proxy lifecycle

**Files:**
- Create: `lane/internal/proxy/traefik-compose.yml.tmpl`, `lane/internal/proxy/proxy.go`
- Test: `lane/internal/proxy/proxy_test.go`

> The template lives **inside** `internal/proxy/` so `//go:embed` can reference it directly — `embed` forbids parent (`../`) paths.

- [ ] **Step 1: Create the embedded Traefik compose template**

`internal/proxy/traefik-compose.yml.tmpl`:
```yaml
services:
  proxy:
    image: traefik:v3.1
    container_name: lane-proxy
    command:
      - --providers.docker=true
      - --providers.docker.exposedByDefault=false
      - --providers.docker.network={{.Network}}
      - --providers.file.directory=/dynamic
      - --providers.file.watch=true
      - --entryPoints.web.address=:80
      - --api.dashboard=true
      - --api.insecure=true
    labels:
      - lane.managed=true
      - lane.proxy=true
    ports:
      - "80:80"
      - "127.0.0.1:8080:8080"
    extra_hosts:
      - "host.docker.internal:host-gateway"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - {{.DynamicDir}}:/dynamic:ro
    networks:
      - {{.Network}}
    restart: unless-stopped
networks:
  {{.Network}}:
    name: {{.Network}}
    external: true
```

- [ ] **Step 2: Write the failing test**

`internal/proxy/proxy_test.go`:
```go
package proxy

import (
	"strings"
	"testing"
)

func TestRenderCompose(t *testing.T) {
	out, err := renderCompose("lane", "/tmp/lane/traefik/dynamic")
	if err != nil {
		t.Fatalf("render error: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"image: traefik:v3.1",
		"--providers.docker.network=lane",
		"host.docker.internal:host-gateway",
		"/tmp/lane/traefik/dynamic:/dynamic:ro",
		"external: true",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rendered compose missing %q\n%s", want, s)
		}
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/proxy/`
Expected: FAIL — `undefined: renderCompose`.

- [ ] **Step 4: Implement**

`internal/proxy/proxy.go`:
```go
// Package proxy manages the shared Traefik container and the lane network.
package proxy

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/dheeraj-nalapat/lane/internal/paths"
)

//go:embed traefik-compose.yml.tmpl
var composeTmpl string

// Network is the shared external Docker network name.
const Network = "lane"

func renderCompose(network, dynamicDir string) ([]byte, error) {
	t, err := template.New("c").Parse(composeTmpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, map[string]string{"Network": network, "DynamicDir": dynamicDir})
	return buf.Bytes(), err
}

func composePath() string { return filepath.Join(paths.Traefik(), "docker-compose.yml") }

// ensureNetwork creates the external network if it does not exist.
func ensureNetwork() error {
	// `docker network inspect` exits non-zero when missing.
	if exec.Command("docker", "network", "inspect", Network).Run() == nil {
		return nil
	}
	out, err := exec.Command("docker", "network", "create", Network).CombinedOutput()
	if err != nil {
		return fmt.Errorf("creating %s network: %v\n%s", Network, err, out)
	}
	return nil
}

// Up ensures paths, network, the rendered compose file, and starts Traefik.
func Up() error {
	if err := paths.Ensure(); err != nil {
		return err
	}
	if err := ensureNetwork(); err != nil {
		return err
	}
	body, err := renderCompose(Network, paths.TraefikDynamic())
	if err != nil {
		return err
	}
	if err := os.WriteFile(composePath(), body, 0o644); err != nil {
		return err
	}
	return dockerCompose("up", "-d")
}

// Down stops Traefik (leaves the network in place).
func Down() error { return dockerCompose("down") }

// Running reports whether the lane-proxy container is up.
func Running() bool {
	out, _ := exec.Command("docker", "ps", "--filter", "name=^lane-proxy$", "--format", "{{.Names}}").Output()
	return bytes.Contains(out, []byte("lane-proxy"))
}

// Ensure starts the proxy only if it is not already running.
func Ensure() error {
	if Running() {
		return nil
	}
	return Up()
}

func dockerCompose(args ...string) error {
	full := append([]string{"compose", "-p", "lane-proxy", "-f", composePath()}, args...)
	cmd := exec.Command("docker", full...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
```

- [ ] **Step 5: Run test**

Run: `go test ./internal/proxy/`
Expected: PASS.

- [ ] **Step 6: Manual verification (needs Docker)**

Add a temporary `proxy` command (or test by wiring Task 14 first). For now verify the rendered file and that Docker accepts it:
```bash
go run . proxy up      # after Task 14 wires the command; else skip to Task 14
```

- [ ] **Step 7: Commit**

```bash
git add internal/proxy/
git commit -m "feat: shared Traefik proxy lifecycle and lane network"
```

---

## Phase 4 — up / down / proxy commands

### Task 10: `lane proxy` command

**Files:**
- Create: `lane/cmd/proxy.go`

- [ ] **Step 1: Implement the command**

`cmd/proxy.go`:
```go
package cmd

import (
	"fmt"

	"github.com/dheeraj-nalapat/lane/internal/proxy"
	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use:   "proxy [up|down|status]",
	Short: "Manage the shared Traefik proxy",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "up":
			if err := proxy.Up(); err != nil {
				return err
			}
			fmt.Println("lane proxy is up (http://*.localhost → your stacks)")
			return nil
		case "down":
			return proxy.Down()
		case "status":
			if proxy.Running() {
				fmt.Println("running")
			} else {
				fmt.Println("stopped")
			}
			return nil
		default:
			return fmt.Errorf("unknown subcommand %q (use up|down|status)", args[0])
		}
	},
}

func init() { root.AddCommand(proxyCmd) }
```

- [ ] **Step 2: Build + manual verify (needs Docker)**

```bash
go build ./... && go run . proxy up && go run . proxy status
```
Expected: `lane proxy is up …`, then `running`. Confirm with `docker ps | grep lane-proxy`. Visit `http://localhost:8080/dashboard/` — Traefik dashboard loads.

- [ ] **Step 3: Commit**

```bash
git add cmd/proxy.go
git commit -m "feat: lane proxy up|down|status command"
```

---

### Task 11: Tilt dynamic route + tilt command builder

**Files:**
- Create: `lane/internal/tiltx/tiltx.go`
- Test: `lane/internal/tiltx/tiltx_test.go`

- [ ] **Step 1: Write the failing test**

`internal/tiltx/tiltx_test.go`:
```go
package tiltx

import (
	"strings"
	"testing"
)

func TestRenderDynamic(t *testing.T) {
	out, err := RenderDynamicRoute("remind", 10377)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"Host(`tilt-remind.localhost`)",
		"http://host.docker.internal:10377",
		"remind-tilt",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
}

func TestUpArgs(t *testing.T) {
	got := UpArgs(10377)
	want := []string{"up", "--", "--docker", "--port", "10377"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("UpArgs = %v, want %v", got, want)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tiltx/`
Expected: FAIL — `undefined: RenderDynamicRoute`.

- [ ] **Step 3: Implement**

`internal/tiltx/tiltx.go`:
```go
// Package tiltx builds Tilt invocations and the Traefik file-provider route
// that fronts the per-stack Tilt dashboard.
package tiltx

import (
	"bytes"
	"strconv"
	"text/template"
)

const dynamicTmpl = `http:
  routers:
    {{.Slug}}-tilt:
      rule: "Host(` + "`tilt-{{.Slug}}.localhost`" + `)"
      service: {{.Slug}}-tilt
      entryPoints: [web]
  services:
    {{.Slug}}-tilt:
      loadBalancer:
        servers:
          - url: "http://host.docker.internal:{{.Port}}"
`

// RenderDynamicRoute produces the Traefik file-provider config routing
// tilt-<slug>.localhost to the host-side Tilt UI port.
func RenderDynamicRoute(slug string, port int) ([]byte, error) {
	t, err := template.New("d").Parse(dynamicTmpl)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = t.Execute(&buf, map[string]any{"Slug": slug, "Port": port})
	return buf.Bytes(), err
}

// UpArgs returns the args for `tilt up` in lane's docker mode on a given port.
func UpArgs(port int) []string {
	return []string{"up", "--", "--docker", "--port", strconv.Itoa(port)}
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/tiltx/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tiltx/
git commit -m "feat: tilt up args and tilt-<slug>.localhost dynamic route"
```

---

### Task 12: `lane up`

**Files:**
- Create: `lane/cmd/up.go`, `lane/internal/identity/identity.go`
- Test: `lane/internal/identity/identity_test.go`

`identity` packages the resolve-everything step so it is unit-testable without running Tilt.

- [ ] **Step 1: Write the failing test for identity resolution**

`internal/identity/identity_test.go`:
```go
package identity

import "testing"

func TestResolveHostnames(t *testing.T) {
	got := RenderHost("api.{slug}", "remind-featx")
	if got != "api.remind-featx.localhost" {
		t.Fatalf("RenderHost = %q", got)
	}
	if RenderHost("{slug}", "remind") != "remind.localhost" {
		t.Fatalf("default host wrong")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/identity/`
Expected: FAIL — `undefined: RenderHost`.

- [ ] **Step 3: Implement identity helper**

`internal/identity/identity.go`:
```go
// Package identity renders hostnames from route templates and a slug.
package identity

import "strings"

// RenderHost expands a host template like "{slug}" or "api.{slug}" into a full
// "<...>.localhost" hostname.
func RenderHost(tmpl, slug string) string {
	return strings.ReplaceAll(tmpl, "{slug}", slug) + ".localhost"
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/identity/`
Expected: PASS.

- [ ] **Step 5: Implement the `up` command (wires everything)**

`cmd/up.go`:
```go
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dheeraj-nalapat/lane/internal/compose"
	"github.com/dheeraj-nalapat/lane/internal/dockerx"
	"github.com/dheeraj-nalapat/lane/internal/gitx"
	"github.com/dheeraj-nalapat/lane/internal/identity"
	"github.com/dheeraj-nalapat/lane/internal/manifest"
	"github.com/dheeraj-nalapat/lane/internal/override"
	"github.com/dheeraj-nalapat/lane/internal/paths"
	"github.com/dheeraj-nalapat/lane/internal/ports"
	"github.com/dheeraj-nalapat/lane/internal/proxy"
	"github.com/dheeraj-nalapat/lane/internal/slug"
	"github.com/dheeraj-nalapat/lane/internal/tiltx"
	"github.com/spf13/cobra"
)

var flagDetach bool

var upCmd = &cobra.Command{
	Use:   "up [path]",
	Short: "Bring a stack up behind the lane proxy",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runUp,
}

func init() {
	upCmd.Flags().BoolVarP(&flagDetach, "detach", "d", false, "run Tilt in the background")
	root.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	dir, err := projectDir(args)
	if err != nil {
		return err
	}

	m, err := manifest.Load(filepath.Join(dir, ".lane.toml"))
	if err != nil {
		return err
	}

	wt, _, err := gitx.Worktree(dir)
	if err != nil {
		return err
	}
	sl := slug.Resolve(slug.Inputs{
		Flag: flagSlug, Env: os.Getenv("LANE_SLUG"),
		ManifestName: m.Name, Worktree: wt, DirBase: filepath.Base(dir),
	})

	// Collision: same slug already claimed by a different path?
	if claimed, ok := dockerx.SlugOwner(sl); ok && claimed != dir {
		return fmt.Errorf("slug %q already in use by stack at %s; pass --slug to disambiguate", sl, claimed)
	}

	tiltPort, err := ports.Free()
	if err != nil {
		return err
	}

	composePath := filepath.Join(dir, m.ComposeFile)
	svcs, err := compose.ServiceNames(composePath)
	if err != nil {
		return err
	}

	var routes []override.Route
	for _, r := range m.Routes {
		routes = append(routes, override.Route{
			Service: r.Service, Port: r.Port,
			Hostname: identity.RenderHost(r.Host, sl),
		})
	}

	body, err := override.Generate(override.Spec{
		Slug: sl, ProjectPath: dir, Network: proxy.Network,
		Services: svcs, Routes: routes, TiltPort: tiltPort,
	})
	if err != nil {
		return err
	}
	if err := paths.Ensure(); err != nil {
		return err
	}
	overridePath := filepath.Join(paths.Overrides(), sl+".override.yml")
	dynamicPath := filepath.Join(paths.TraefikDynamic(), sl+".yml")
	dynamic, err := tiltx.RenderDynamicRoute(sl, tiltPort)
	if err != nil {
		return err
	}

	env := append(os.Environ(),
		"COMPOSE_PROJECT_NAME="+sl,
		"LANE=1",
		"LANE_SLUG="+sl,
		"LANE_COMPOSE_OVERRIDE="+overridePath,
	)
	if m.APITarget != "" {
		env = append(env, "LANE_API_TARGET=http://"+m.APITarget)
	}

	if flagDryRun {
		fmt.Printf("# slug: %s\n# tilt port: %d\n# override (%s):\n%s\n# tilt dynamic (%s):\n%s\n# command: tilt %v\n# env adds: COMPOSE_PROJECT_NAME, LANE, LANE_SLUG, LANE_COMPOSE_OVERRIDE\n",
			sl, tiltPort, overridePath, body, dynamicPath, dynamic, tiltx.UpArgs(tiltPort))
		return nil
	}

	if err := os.WriteFile(overridePath, body, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(dynamicPath, dynamic, 0o644); err != nil {
		return err
	}
	if err := proxy.Ensure(); err != nil {
		return err
	}

	printURLs(sl, routes)

	tcmd := exec.Command("tilt", tiltx.UpArgs(tiltPort)...)
	tcmd.Dir = dir
	tcmd.Env = env

	if flagDetach {
		logf, err := os.Create(filepath.Join(paths.Run(), sl+".log"))
		if err != nil {
			return err
		}
		tcmd.Stdout, tcmd.Stderr = logf, logf
		if err := tcmd.Start(); err != nil {
			return err
		}
		pidFile := filepath.Join(paths.Run(), sl+".pid")
		_ = os.WriteFile(pidFile, []byte(fmt.Sprint(tcmd.Process.Pid)), 0o644)
		fmt.Printf("detached (pid %d). logs: lane logs --slug %s\n", tcmd.Process.Pid, sl)
		return nil
	}

	tcmd.Stdout, tcmd.Stderr, tcmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	return tcmd.Run()
}

func projectDir(args []string) (string, error) {
	if len(args) == 1 {
		return filepath.Abs(args[0])
	}
	return os.Getwd()
}

func printURLs(sl string, routes []override.Route) {
	fmt.Printf("lane: %s\n", sl)
	for _, r := range routes {
		fmt.Printf("  → http://%s  (%s:%d)\n", r.Hostname, r.Service, r.Port)
	}
	fmt.Printf("  → http://tilt-%s.localhost  (Tilt UI)\n", sl)
}
```

- [ ] **Step 6: Build**

Run: `go build ./...`
Expected: builds once `dockerx.SlugOwner` (Task 13) exists. **Implement Task 13 before building this**, or temporarily stub `dockerx.SlugOwner` returning `("", false)`.

- [ ] **Step 7: Commit**

```bash
git add cmd/up.go internal/identity/
git commit -m "feat: lane up — resolve slug, generate override, run tilt"
```

---

### Task 13: Docker label queries (Stack inventory)

**Files:**
- Create: `lane/internal/stack/stack.go`, `lane/internal/dockerx/dockerx.go`
- Test: `lane/internal/dockerx/dockerx_test.go`

- [ ] **Step 1: Write the failing test (pure parsing)**

`internal/dockerx/dockerx_test.go`:
```go
package dockerx

import "testing"

func TestParsePS(t *testing.T) {
	// One JSON line per container, as `docker ps --format '{{json .}}'` emits.
	lines := `{"Names":"remind-ui-1","Labels":"lane.managed=true,lane.slug=remind,lane.url=http://remind.localhost,lane.tilt.port=10377,lane.project.path=/home/u/remind","State":"running"}
{"Names":"remind-server-1","Labels":"lane.managed=true,lane.slug=remind,lane.project.path=/home/u/remind","State":"running"}
{"Names":"x-ui-1","Labels":"lane.managed=true,lane.slug=x,lane.url=http://x.localhost,lane.tilt.port=10500,lane.project.path=/home/u/x","State":"running"}`
	stacks := parsePS([]byte(lines))
	if len(stacks) != 2 {
		t.Fatalf("got %d stacks, want 2", len(stacks))
	}
	bySlug := map[string]int{}
	for _, s := range stacks {
		bySlug[s.Slug] = s.TiltPort
	}
	if bySlug["remind"] != 10377 {
		t.Fatalf("remind tilt port = %d", bySlug["remind"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/dockerx/`
Expected: FAIL — `undefined: parsePS`.

- [ ] **Step 3: Implement Stack + dockerx**

`internal/stack/stack.go`:
```go
// Package stack holds the shared Stack model used by ls/view/down.
package stack

// Stack is one lane-managed project stack, aggregated from container labels.
type Stack struct {
	Slug        string
	URL         string
	TiltPort    int
	ProjectPath string
	Containers  []string
	Running     bool
}
```

`internal/dockerx/dockerx.go`:
```go
// Package dockerx queries Docker for lane-managed containers.
package dockerx

import (
	"encoding/json"
	"os/exec"
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
			s = &stack.Stack{Slug: sl, ProjectPath: lbl["lane.project.path"]}
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
```

- [ ] **Step 4: Run test + build**

Run: `go test ./internal/dockerx/ && go build ./...`
Expected: PASS; `lane up` now builds.

- [ ] **Step 5: Commit**

```bash
git add internal/stack/ internal/dockerx/
git commit -m "feat: query lane stacks from docker labels"
```

---

### Task 14: `lane down`

**Files:**
- Create: `lane/cmd/down.go`

- [ ] **Step 1: Implement**

`cmd/down.go`:
```go
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/dheeraj-nalapat/lane/internal/gitx"
	"github.com/dheeraj-nalapat/lane/internal/manifest"
	"github.com/dheeraj-nalapat/lane/internal/paths"
	"github.com/dheeraj-nalapat/lane/internal/slug"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down [path]",
	Short: "Tear down a stack and remove its generated files",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runDown,
}

func init() { root.AddCommand(downCmd) }

func runDown(cmd *cobra.Command, args []string) error {
	dir, err := projectDir(args)
	if err != nil {
		return err
	}
	m, err := manifest.Load(filepath.Join(dir, ".lane.toml"))
	if err != nil {
		return err
	}
	wt, _, _ := gitx.Worktree(dir)
	sl := slug.Resolve(slug.Inputs{
		Flag: flagSlug, Env: os.Getenv("LANE_SLUG"),
		ManifestName: m.Name, Worktree: wt, DirBase: filepath.Base(dir),
	})

	// Stop a detached Tilt process if present.
	pidFile := filepath.Join(paths.Run(), sl+".pid")
	if b, err := os.ReadFile(pidFile); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
			_ = syscall.Kill(pid, syscall.SIGTERM)
		}
		_ = os.Remove(pidFile)
	}

	// Tear down compose resources for this project (keeps named volumes).
	overridePath := filepath.Join(paths.Overrides(), sl+".override.yml")
	composePath := filepath.Join(dir, m.ComposeFile)
	dc := exec.Command("docker", "compose", "-p", sl, "-f", composePath, "-f", overridePath, "down", "--remove-orphans")
	dc.Stdout, dc.Stderr = os.Stdout, os.Stderr
	if err := dc.Run(); err != nil {
		return err
	}

	_ = os.Remove(overridePath)
	_ = os.Remove(filepath.Join(paths.TraefikDynamic(), sl+".yml"))
	fmt.Printf("lane: %s torn down\n", sl)
	return nil
}
```

- [ ] **Step 2: Build**

Run: `go build ./...`
Expected: builds.

- [ ] **Step 3: Commit**

```bash
git add cmd/down.go
git commit -m "feat: lane down — teardown + cleanup, repo left pristine"
```

---

## Phase 5 — Inventory and view

### Task 15: `lane ls`

**Files:**
- Create: `lane/cmd/ls.go`

- [ ] **Step 1: Implement**

`cmd/ls.go`:
```go
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/dheeraj-nalapat/lane/internal/dockerx"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List running lane stacks",
	RunE: func(cmd *cobra.Command, args []string) error {
		stacks, err := dockerx.List()
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "SLUG\tURL\tTILT\tSTATE\tPATH")
		for _, s := range stacks {
			state := "stopped"
			if s.Running {
				state = "running"
			}
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", s.Slug, s.URL, s.TiltPort, state, s.ProjectPath)
		}
		return w.Flush()
	},
}

func init() { root.AddCommand(lsCmd) }
```

- [ ] **Step 2: Build + manual verify**

```bash
go build ./... && go run . ls
```
Expected: header row (empty if nothing running).

- [ ] **Step 3: Commit**

```bash
git add cmd/ls.go
git commit -m "feat: lane ls"
```

---

### Task 16: Traefik API client

**Files:**
- Create: `lane/internal/traefikapi/traefikapi.go`
- Test: `lane/internal/traefikapi/traefikapi_test.go`

- [ ] **Step 1: Write the failing test**

`internal/traefikapi/traefikapi_test.go`:
```go
package traefikapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouters(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"name":"remind-ui@docker","rule":"Host(` + "`remind.localhost`" + `)","service":"remind-ui","status":"enabled"}]`))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	rs, err := c.Routers()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(rs) != 1 || rs[0].Rule != "Host(`remind.localhost`)" {
		t.Fatalf("unexpected routers: %+v", rs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/traefikapi/`
Expected: FAIL — `undefined: Client`.

- [ ] **Step 3: Implement**

`internal/traefikapi/traefikapi.go`:
```go
// Package traefikapi reads live routing state from the Traefik API.
package traefikapi

import (
	"encoding/json"
	"net/http"
	"time"
)

// Router is one Traefik HTTP router.
type Router struct {
	Name    string `json:"name"`
	Rule    string `json:"rule"`
	Service string `json:"service"`
	Status  string `json:"status"`
}

// Client talks to the Traefik API (default http://127.0.0.1:8080).
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// Default returns a Client pointed at the local lane proxy API.
func Default() *Client {
	return &Client{BaseURL: "http://127.0.0.1:8080", HTTP: &http.Client{Timeout: 2 * time.Second}}
}

// Routers returns all HTTP routers Traefik currently knows about.
func (c *Client) Routers() ([]Router, error) {
	resp, err := c.HTTP.Get(c.BaseURL + "/api/http/routers")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var rs []Router
	if err := json.NewDecoder(resp.Body).Decode(&rs); err != nil {
		return nil, err
	}
	return rs, nil
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/traefikapi/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/traefikapi/
git commit -m "feat: traefik API client for live routing"
```

---

### Task 17: `lane view` (TUI control panel)

**Files:**
- Create: `lane/cmd/view.go`, `lane/internal/ui/view.go`

- [ ] **Step 1: Add dependencies**

```bash
go get github.com/charmbracelet/bubbletea@latest github.com/charmbracelet/lipgloss@latest
```

- [ ] **Step 2: Implement the renderer**

`internal/ui/view.go`:
```go
// Package ui renders the lane view control panel.
package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/dheeraj-nalapat/lane/internal/stack"
	"github.com/dheeraj-nalapat/lane/internal/traefikapi"
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	slugStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	downStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

// Render builds the static view string from stacks and live Traefik routers.
func Render(stacks []stack.Stack, routers []traefikapi.Router) string {
	routesBySlug := map[string][]traefikapi.Router{}
	for _, r := range routers {
		// router name like "<slug>-<svc>@docker"
		name := strings.SplitN(r.Name, "@", 2)[0]
		if i := strings.LastIndexByte(name, '-'); i > 0 {
			routesBySlug[name[:i]] = append(routesBySlug[name[:i]], r)
		}
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("⚓ lane — running stacks") + "\n\n")
	if len(stacks) == 0 {
		b.WriteString(dimStyle.Render("  (none — run `lane up` in a project)\n"))
		return b.String()
	}
	sort.Slice(stacks, func(i, j int) bool { return stacks[i].Slug < stacks[j].Slug })
	for _, s := range stacks {
		state := slugStyle.Render(s.Slug)
		if !s.Running {
			state = downStyle.Render(s.Slug + " (stopped)")
		}
		b.WriteString(state + "  " + dimStyle.Render(s.ProjectPath) + "\n")
		b.WriteString("  " + s.URL + "\n")
		b.WriteString(fmt.Sprintf("  %s\n", dimStyle.Render(fmt.Sprintf("tilt → http://tilt-%s.localhost (:%d)", s.Slug, s.TiltPort))))
		for _, r := range routesBySlug[s.Slug] {
			mark := "✓"
			if r.Status != "enabled" {
				mark = "✗"
			}
			b.WriteString(fmt.Sprintf("    %s %s → %s\n", mark, r.Rule, r.Service))
		}
		b.WriteString("\n")
	}
	return b.String()
}
```

- [ ] **Step 3: Add a render test**

`internal/ui/view_test.go`:
```go
package ui

import (
	"strings"
	"testing"

	"github.com/dheeraj-nalapat/lane/internal/stack"
	"github.com/dheeraj-nalapat/lane/internal/traefikapi"
)

func TestRender(t *testing.T) {
	out := Render(
		[]stack.Stack{{Slug: "remind", URL: "http://remind.localhost", TiltPort: 10377, ProjectPath: "/p", Running: true}},
		[]traefikapi.Router{{Name: "remind-ui@docker", Rule: "Host(`remind.localhost`)", Service: "remind-ui", Status: "enabled"}},
	)
	if !strings.Contains(out, "remind") || !strings.Contains(out, "remind.localhost") {
		t.Fatalf("render missing content:\n%s", out)
	}
}

func TestRender_Empty(t *testing.T) {
	if !strings.Contains(Render(nil, nil), "none") {
		t.Fatal("empty render should say none")
	}
}
```

Run: `go test ./internal/ui/`
Expected: PASS.

- [ ] **Step 4: Implement the command (static + --watch)**

`cmd/view.go`:
```go
package cmd

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/dheeraj-nalapat/lane/internal/dockerx"
	"github.com/dheeraj-nalapat/lane/internal/traefikapi"
	"github.com/dheeraj-nalapat/lane/internal/ui"
	"github.com/spf13/cobra"
)

var flagWatch bool

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "Rich control panel of running stacks and live routing",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !flagWatch {
			fmt.Print(snapshot())
			return nil
		}
		_, err := tea.NewProgram(model{}).Run()
		return err
	},
}

func init() {
	viewCmd.Flags().BoolVar(&flagWatch, "watch", false, "live-refresh the view")
	root.AddCommand(viewCmd)
}

func snapshot() string {
	stacks, _ := dockerx.List()
	routers, _ := traefikapi.Default().Routers()
	return ui.Render(stacks, routers)
}

type tick struct{}
type model struct{ body string }

func (m model) Init() tea.Cmd { return tea.Batch(refresh, tea.EnterAltScreen) }

func refresh() tea.Msg { return tick{} }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tick:
		m.body = snapshot()
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tick{} })
	case tea.KeyMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m model) View() string { return m.body + "\n(press any key to quit)\n" }
```

- [ ] **Step 5: Build + manual verify**

```bash
go build ./... && go run . view
go run . view --watch    # press a key to exit
```

- [ ] **Step 6: Commit**

```bash
git add cmd/view.go internal/ui/ go.mod go.sum
git commit -m "feat: lane view control panel (static + --watch)"
```

---

## Phase 6 — Helper commands

### Task 18: `lane doctor`

**Files:**
- Create: `lane/cmd/doctor.go`, `lane/internal/doctor/doctor.go`
- Test: `lane/internal/doctor/doctor_test.go`

- [ ] **Step 1: Write the failing test (version parse)**

`internal/doctor/doctor_test.go`:
```go
package doctor

import "testing"

func TestComposeOK(t *testing.T) {
	cases := map[string]bool{
		"Docker Compose version v2.40.1": true,
		"Docker Compose version v2.20.0": true,
		"Docker Compose version v2.19.9": false,
		"Docker Compose version v1.29.2": false,
		"garbage":                        false,
	}
	for in, want := range cases {
		if got := composeOK(in); got != want {
			t.Errorf("composeOK(%q) = %v, want %v", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/doctor/`
Expected: FAIL — `undefined: composeOK`.

- [ ] **Step 3: Implement**

`internal/doctor/doctor.go`:
```go
// Package doctor runs lane preflight checks.
package doctor

import (
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// Check is one diagnostic result.
type Check struct {
	Name string
	OK   bool
	Hint string
}

var verRe = regexp.MustCompile(`v(\d+)\.(\d+)\.\d+`)

// composeOK reports whether a `docker compose version` line is >= 2.20.
func composeOK(line string) bool {
	mm := verRe.FindStringSubmatch(line)
	if mm == nil {
		return false
	}
	major, _ := strconv.Atoi(mm[1])
	minor, _ := strconv.Atoi(mm[2])
	return major > 2 || (major == 2 && minor >= 20)
}

func cmdOut(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// Run executes all checks.
func Run() []Check {
	var checks []Check

	_, err := cmdOut("docker", "info")
	checks = append(checks, Check{"docker daemon", err == nil, "start Docker"})

	cv, _ := cmdOut("docker", "compose", "version")
	checks = append(checks, Check{"compose >= 2.20", composeOK(cv), "upgrade Docker Compose"})

	_, err = exec.LookPath("tilt")
	checks = append(checks, Check{"tilt installed", err == nil, "install Tilt: https://tilt.dev"})

	// *.localhost resolves to loopback?
	addrs, _ := net.LookupHost("lane-check.localhost")
	loop := false
	for _, a := range addrs {
		if a == "127.0.0.1" || a == "::1" {
			loop = true
		}
	}
	checks = append(checks, Check{"*.localhost → loopback", loop, "ensure your resolver maps .localhost to 127.0.0.1"})

	return checks
}

// Report formats the checks; returns (text, allOK).
func Report() (string, bool) {
	var b strings.Builder
	all := true
	for _, c := range Run() {
		mark := "✓"
		if !c.OK {
			mark, all = "✗", false
			b.WriteString(fmt.Sprintf("%s %s — %s\n", mark, c.Name, c.Hint))
		} else {
			b.WriteString(fmt.Sprintf("%s %s\n", mark, c.Name))
		}
	}
	return b.String(), all
}
```

- [ ] **Step 4: Implement the command**

`cmd/doctor.go`:
```go
package cmd

import (
	"errors"
	"fmt"

	"github.com/dheeraj-nalapat/lane/internal/doctor"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check that the environment is ready for lane",
	RunE: func(cmd *cobra.Command, args []string) error {
		report, ok := doctor.Report()
		fmt.Print(report)
		if !ok {
			return errors.New("some checks failed")
		}
		return nil
	},
}

func init() { root.AddCommand(doctorCmd) }
```

- [ ] **Step 5: Run tests + manual verify**

```bash
go test ./internal/doctor/ && go build ./... && go run . doctor
```
Expected: PASS; doctor prints checks (✓ for docker/compose/tilt/localhost on this machine).

- [ ] **Step 6: Commit**

```bash
git add cmd/doctor.go internal/doctor/
git commit -m "feat: lane doctor preflight checks"
```

---

### Task 19: `lane init`

**Files:**
- Create: `lane/cmd/init.go`, `lane/internal/scaffold/scaffold.go`
- Test: `lane/internal/scaffold/scaffold_test.go`

- [ ] **Step 1: Write the failing test**

`internal/scaffold/scaffold_test.go`:
```go
package scaffold

import (
	"strings"
	"testing"
)

func TestGuessAndRender(t *testing.T) {
	compose := `services:
  server:
    ports: ["8000:8000"]
  ui:
    ports: ["80:80"]
`
	svc, port := GuessWebEntry(compose)
	if svc != "ui" || port != 80 {
		t.Fatalf("guessed %s:%d, want ui:80", svc, port)
	}
	out := RenderManifest("myproj", "docker-compose.yml", svc, port)
	for _, want := range []string{`name = "myproj"`, `service = "ui"`, "port = 80"} {
		if !strings.Contains(out, want) {
			t.Errorf("manifest missing %q:\n%s", want, out)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/scaffold/`
Expected: FAIL — `undefined: GuessWebEntry`.

- [ ] **Step 3: Implement**

`internal/scaffold/scaffold.go`:
```go
// Package scaffold powers `lane init`: guess the web entrypoint and render
// a starter .lane.toml.
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

// RenderManifest produces .lane.toml content.
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
```

- [ ] **Step 4: Implement the command**

`cmd/init.go`:
```go
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dheeraj-nalapat/lane/internal/scaffold"
	"github.com/spf13/cobra"
)

var flagCompose string

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Scaffold a .lane.toml for the current project",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, _ := os.Getwd()
		manifestPath := filepath.Join(dir, ".lane.toml")
		if _, err := os.Stat(manifestPath); err == nil {
			return errors.New(".lane.toml already exists")
		}

		composeRel := flagCompose
		if composeRel == "" {
			for _, c := range []string{"docker-compose.yml", "infra/docker-compose.yml", "compose.yaml"} {
				if _, err := os.Stat(filepath.Join(dir, c)); err == nil {
					composeRel = c
					break
				}
			}
		}
		if composeRel == "" {
			return errors.New("no compose file found; pass --compose <path>")
		}

		body, err := os.ReadFile(filepath.Join(dir, composeRel))
		if err != nil {
			return err
		}
		svc, port := scaffold.GuessWebEntry(string(body))
		if svc == "" {
			return errors.New("could not guess a web entrypoint; edit .lane.toml manually after creation")
		}
		out := scaffold.RenderManifest(filepath.Base(dir), composeRel, svc, port)
		if err := os.WriteFile(manifestPath, []byte(out), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote .lane.toml (routing %s:%d). Review it, then `lane up`.\n", svc, port)
		return nil
	},
}

func init() {
	initCmd.Flags().StringVar(&flagCompose, "compose", "", "path to the base compose file")
	root.AddCommand(initCmd)
}
```

- [ ] **Step 5: Run tests + build**

Run: `go test ./internal/scaffold/ && go build ./...`
Expected: PASS, builds.

- [ ] **Step 6: Commit**

```bash
git add cmd/init.go internal/scaffold/
git commit -m "feat: lane init scaffolds .lane.toml"
```

---

### Task 20: `lane open` and `lane logs`

**Files:**
- Create: `lane/cmd/open.go`, `lane/cmd/logs.go`

- [ ] **Step 1: Implement `open`**

`cmd/open.go`:
```go
package cmd

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/dheeraj-nalapat/lane/internal/dockerx"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open a stack's URL in the browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		stacks, err := dockerx.List()
		if err != nil {
			return err
		}
		var url string
		for _, s := range stacks {
			if (flagSlug == "" || s.Slug == flagSlug) && s.URL != "" {
				url = s.URL
				break
			}
		}
		if url == "" {
			return fmt.Errorf("no running stack with a URL (use --slug)")
		}
		opener := "xdg-open"
		if runtime.GOOS == "darwin" {
			opener = "open"
		}
		return exec.Command(opener, url).Start()
	},
}

func init() { root.AddCommand(openCmd) }
```

- [ ] **Step 2: Implement `logs`**

`cmd/logs.go`:
```go
package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dheeraj-nalapat/lane/internal/gitx"
	"github.com/dheeraj-nalapat/lane/internal/manifest"
	"github.com/dheeraj-nalapat/lane/internal/paths"
	"github.com/dheeraj-nalapat/lane/internal/slug"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs [path]",
	Short: "Tail a stack's logs",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := projectDir(args)
		if err != nil {
			return err
		}
		sl := flagSlug
		if sl == "" {
			m, err := manifest.Load(filepath.Join(dir, ".lane.toml"))
			if err != nil {
				return err
			}
			wt, _, _ := gitx.Worktree(dir)
			sl = slug.Resolve(slug.Inputs{ManifestName: m.Name, Worktree: wt, DirBase: filepath.Base(dir), Env: os.Getenv("LANE_SLUG")})
		}

		// Detached run → tail the log file; else stream compose logs.
		logFile := filepath.Join(paths.Run(), sl+".log")
		if _, err := os.Stat(logFile); err == nil {
			c := exec.Command("tail", "-f", logFile)
			c.Stdout, c.Stderr = os.Stdout, os.Stderr
			return c.Run()
		}
		fmt.Printf("no detached log for %s; streaming container logs\n", sl)
		c := exec.Command("docker", "compose", "-p", sl, "logs", "-f")
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		return c.Run()
	},
}

func init() { root.AddCommand(logsCmd) }
```

- [ ] **Step 3: Build**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: builds, vet clean, all unit tests pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/open.go cmd/logs.go
git commit -m "feat: lane open and lane logs"
```

---

## Phase 7 — Onboard ReMind and verify end-to-end

This is the real-world acceptance test: two worktrees of ReMind running at once, both reachable.

### Task 21: Make ReMind's Tiltfile lane-aware + add manifest

**Files:**
- Modify: `/home/dheerajnalapat/project/ReMind/Tiltfile` (docker branch only)
- Create: `/home/dheerajnalapat/project/ReMind/.lane.toml`
- Modify: `/home/dheerajnalapat/project/ReMind/.gitignore` (ignore nothing new — overrides live in ~/.lane)

- [ ] **Step 1: Add the manifest**

`/home/dheerajnalapat/project/ReMind/.lane.toml`:
```toml
name = "remind"
compose_file = "infra/docker-compose.yml"

[[routes]]
service = "ui"
port = 80
```

(ReMind's docker-mode `ui` is a static nginx build on :80 — lane just routes it. No Vite/HMR changes; that's the service-agnostic design.)

- [ ] **Step 2: Make the Tiltfile pick up lane's override + slug-namespaced images**

In `/home/dheerajnalapat/project/ReMind/Tiltfile`, replace the `docker_compose("./infra/docker-compose.yml")` call inside the `if use_docker:` branch with the lane-aware shim, and namespace the built image tags by slug:

```python
if use_docker:
    # --- lane integration (active only under `lane up`) ---
    lane_slug = os.getenv("LANE_SLUG", "")
    tag = (":" + lane_slug) if lane_slug else ""

    compose_files = ["./infra/docker-compose.yml"]
    lane_override = os.getenv("LANE_COMPOSE_OVERRIDE", "")
    if lane_override:
        compose_files.append(lane_override)
    docker_compose(compose_files)

    docker_build(
        "remind/platform-server" + tag,
        context=".",
        dockerfile="infra/docker/Dockerfile.server",
        live_update=[
            sync("./services/platform/src", "/app/src"),
            run("uv pip install -e .", trigger=["./services/platform/pyproject.toml"]),
            restart_container(),
        ],
    )

    docker_build(
        "remind/agent-server" + tag,
        context=".",
        dockerfile="infra/docker/Dockerfile.agent-server",
        live_update=[
            sync("./services/agent/src", "/app/src"),
            run("uv pip install -e .", trigger=["./services/agent/pyproject.toml"]),
            restart_container(),
        ],
    )

    dc_resource("server", labels=["backend"])
    dc_resource("agent-server", labels=["backend"])
    dc_resource("worker", labels=["backend"])
    dc_resource("ui", labels=["frontend"])
```

If image tags are namespaced, the override must also set each built service's `image:` to the slug tag. Extend `override.Generate` to accept an optional `ImageTag string` and, for services in a provided `BuiltServices` set, emit `image: "<base>:<slug>"`. **However**, since the base image name lives in the compose `image:` field, the simplest correct approach for v1 is: have the Tiltfile set the tag (as above) AND add `image` overrides in `.lane.toml` is overkill — instead, document that for full image isolation the project sets `image: remind/platform-server:${LANE_SLUG:-latest}` in its compose. For ReMind v1, accept shared image tags (live_update syncs into each stack's own containers; full-rebuild cross-contamination is a known, documented v1 limitation — see spec "Deferred"). Keep the Tiltfile `tag` wiring so enabling it later is a one-line change.

> **Decision for v1:** ship without per-slug image tags wired all the way through (keep the Tiltfile hook in place, default tag empty). Add a line to the spec's "Deferred" section: *"Per-slug image-tag isolation — hook present in Tiltfile, not enabled by default in v1."* This keeps the task tractable and honest.

- [ ] **Step 3: Update the spec's Deferred section**

Add to `docs/2026-06-08-lane-design.md` under "Deferred (post-v1)":
```markdown
- **Per-slug image-tag isolation** — the Tiltfile hook (`:${LANE_SLUG}`) is in
  place but disabled by default in v1. Until enabled, two worktrees share built
  image tags; live_update isolates active edits, but a simultaneous full rebuild
  in one worktree can update the shared tag the other uses. Enable by setting
  per-slug `image:` tags in compose + the Tiltfile tag var.
```

- [ ] **Step 4: Commit (lane repo) the spec update**

```bash
cd /home/dheerajnalapat/project/lane
git add docs/2026-06-08-lane-design.md
git commit -m "docs: note per-slug image-tag isolation deferred to post-v1"
```

(ReMind's own changes are committed in the ReMind repo, separately, by the user.)

---

### Task 22: End-to-end verification — two worktrees at once

**No new files. Acceptance test.**

- [ ] **Step 1: Build + install the binary**

```bash
cd /home/dheerajnalapat/project/lane
go build -o /usr/local/bin/lane .   # or: go install .
lane doctor
```
Expected: all checks ✓.

- [ ] **Step 2: Start the proxy and bring up main ReMind**

```bash
lane proxy up
cd /home/dheerajnalapat/project/ReMind
lane up -d
lane ls
```
Expected: `lane ls` shows `remind` with URL `http://remind.localhost`. Visit `http://remind.localhost` and `http://tilt-remind.localhost` — both load.

- [ ] **Step 3: Create a worktree and bring it up simultaneously**

```bash
cd /home/dheerajnalapat/project/ReMind
git worktree add ../remind-featx -b featx
cd ../remind-featx
lane up -d
lane ls
```
Expected: `lane ls` shows BOTH `remind` and `remind-featx`, each with its own URL. Visit `http://remind.localhost` and `http://remind-featx.localhost` — both load **at the same time**, no port conflict. `lane view` shows both stacks and their live Traefik routes.

- [ ] **Step 4: Verify isolation and teardown**

```bash
lane view              # confirm two independent stacks + routes
cd /home/dheerajnalapat/project/remind-featx && lane down
cd /home/dheerajnalapat/project/ReMind && lane down
lane ls                # empty
git -C /home/dheerajnalapat/project/ReMind status --short   # clean: no committed files changed by lane
```
Expected: both torn down; `lane ls` empty; ReMind working tree clean (proves non-invasiveness — overrides lived in ~/.lane).

- [ ] **Step 5: Commit a short acceptance note**

```bash
cd /home/dheerajnalapat/project/lane
mkdir -p docs
printf '%s\n' "# Acceptance: ReMind two-worktree run verified $(date +%F)" > docs/acceptance-remind.md
git add docs/acceptance-remind.md
git commit -m "docs: record ReMind two-worktree acceptance"
```

---

## Phase 8 — Distribution

### Task 23: GoReleaser + install script

**Files:**
- Create: `lane/.goreleaser.yaml`, `lane/install.sh`, `lane/.github/workflows/release.yml`

- [ ] **Step 1: GoReleaser config**

`.goreleaser.yaml`:
```yaml
version: 2
builds:
  - main: .
    binary: lane
    env: [CGO_ENABLED=0]
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
archives:
  - formats: [tar.gz]
    name_template: "lane_{{ .Os }}_{{ .Arch }}"
brews:
  - repository:
      owner: dheerajnalapat        # adjust to real tap repo
      name: homebrew-lane
    homepage: "https://github.com/dheeraj-nalapat/lane"
    description: "Run many project stacks at once with zero port conflicts"
checksum:
  name_template: "checksums.txt"
```

- [ ] **Step 2: Install script**

`install.sh`:
```bash
#!/usr/bin/env sh
set -e
REPO="dheeraj-nalapat/lane"   # adjust
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m); [ "$ARCH" = "x86_64" ] && ARCH=amd64; [ "$ARCH" = "aarch64" ] && ARCH=arm64
URL="https://github.com/$REPO/releases/latest/download/lane_${OS}_${ARCH}.tar.gz"
TMP=$(mktemp -d)
echo "downloading $URL"
curl -sSL "$URL" | tar -xz -C "$TMP"
install -m 0755 "$TMP/lane" /usr/local/bin/lane
echo "installed lane to /usr/local/bin/lane"
```

- [ ] **Step 3: CI workflow**

`.github/workflows/release.yml`:
```yaml
name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - uses: actions/setup-go@v5
        with: { go-version: "1.22" }
      - uses: goreleaser/goreleaser-action@v6
        with: { args: release --clean }
        env: { GITHUB_TOKEN: "${{ secrets.GITHUB_TOKEN }}" }
```

- [ ] **Step 4: Validate config locally**

```bash
go install github.com/goreleaser/goreleaser/v2@latest
goreleaser check
goreleaser build --snapshot --clean   # builds all targets without publishing
```
Expected: `goreleaser check` passes; snapshot builds produce `dist/` binaries.

- [ ] **Step 5: Commit**

```bash
git add .goreleaser.yaml install.sh .github/
git commit -m "build: GoReleaser release pipeline + install script"
```

---

## Final verification

- [ ] `go test ./...` — all unit tests pass.
- [ ] `go vet ./...` — clean.
- [ ] `lane doctor` — green on the dev machine.
- [ ] Phase 7 acceptance (two ReMind worktrees reachable simultaneously) passed.
- [ ] `git -C ReMind status` clean after teardown (non-invasiveness proven).
