# Runner Abstraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let `lane up` drive **plain docker-compose projects with no Tiltfile** (and no shim), while keeping the existing Tilt path, selected automatically.

**Architecture:** Introduce `internal/runner` with a `Runner` interface and two implementations — `tiltRunner` (today's behavior, moved behind the interface) and `composeRunner` (`docker compose -p <slug> -f base -f override up -d`). `cmd/up.go` does the shared prep (slug → override → proxy), selects the runner (`.lane.toml` `runner` > Tiltfile present > compose), and dispatches. The override generator omits the Tilt-UI port label for compose stacks; `view`/`ls` guard on it.

**Tech Stack:** Go 1.22, cobra, gopkg.in/yaml.v3 (existing). No new dependencies.

Spec: `docs/2026-06-09-runner-abstraction-design.md`.

---

## File Structure

```
internal/runner/            NEW package
  runner.go                 RunSpec, Runner interface, Select(), New(), printURLs()
  runner_test.go            Select() table test
  compose.go                composeRunner + buildComposeArgs()
  compose_test.go           buildComposeArgs() test
  tilt.go                   tiltRunner (logic moved out of cmd/up.go)
internal/manifest/manifest.go   + Runner field + validation
internal/override/override.go   omit lane.tilt.port label when TiltPort==0
internal/ui/view.go             render Tilt row only when TiltPort>0
cmd/ls.go                       show "-" in TILT column when port==0
cmd/up.go                       rewired: select runner, conditional tilt port, dispatch, --build
README.md, docs/onboarding-remind.md   note Tiltfile shim is Tilt-only
```

---

### Task 1: Manifest `runner` field + validation

**Files:**
- Modify: `internal/manifest/manifest.go`
- Test: `internal/manifest/manifest_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/manifest/manifest_test.go`:
```go
func TestLoad_RunnerValid(t *testing.T) {
	p := write(t, `name = "x"
compose_file = "docker-compose.yml"
runner = "compose"
[[routes]]
service = "ui"
port = 80
`)
	m, err := Load(p)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if m.Runner != "compose" {
		t.Fatalf("Runner = %q, want compose", m.Runner)
	}
}

func TestLoad_RunnerInvalid(t *testing.T) {
	p := write(t, `name = "x"
compose_file = "docker-compose.yml"
runner = "nomad"
[[routes]]
service = "ui"
port = 80
`)
	if _, err := Load(p); err == nil {
		t.Fatal("expected error for invalid runner")
	}
}

func TestLoad_RunnerDefaultsEmpty(t *testing.T) {
	p := write(t, `name = "x"
compose_file = "docker-compose.yml"
[[routes]]
service = "ui"
port = 80
`)
	m, err := Load(p)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if m.Runner != "" {
		t.Fatalf("Runner = %q, want empty", m.Runner)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/manifest/ -run TestLoad_Runner`
Expected: FAIL — `m.Runner undefined`.

- [ ] **Step 3: Add the field + validation**

In `internal/manifest/manifest.go`, add the field to the `Manifest` struct (after `APITarget`):
```go
	Runner      string  `toml:"runner"`       // "", "tilt", or "compose" (auto if "")
```

In `Load`, after the `len(m.Routes) == 0` check and before the routes loop, add:
```go
	if m.Runner != "" && m.Runner != "tilt" && m.Runner != "compose" {
		return nil, fmt.Errorf(".lane.toml: runner must be \"tilt\" or \"compose\" (got %q)", m.Runner)
	}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/manifest/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/manifest/
git commit -m "feat(manifest): optional runner field with validation"
```

---

### Task 2: `internal/runner` — RunSpec, interface, Select, printURLs

**Files:**
- Create: `internal/runner/runner.go`
- Test: `internal/runner/runner_test.go`

- [ ] **Step 1: Write the failing test**

`internal/runner/runner_test.go`:
```go
package runner

import "testing"

func TestSelect(t *testing.T) {
	cases := []struct {
		name           string
		manifestRunner string
		tiltfile       bool
		want           string
	}{
		{"manifest forces compose", "compose", true, "compose"},
		{"manifest forces tilt", "tilt", false, "tilt"},
		{"auto: tiltfile present", "", true, "tilt"},
		{"auto: no tiltfile", "", false, "compose"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Select(c.manifestRunner, c.tiltfile); got != c.want {
				t.Errorf("Select(%q,%v) = %q, want %q", c.manifestRunner, c.tiltfile, got, c.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	if New("tilt").Name() != "tilt" {
		t.Fatal("New(tilt).Name != tilt")
	}
	if New("compose").Name() != "compose" {
		t.Fatal("New(compose).Name != compose")
	}
	if New("").Name() != "compose" {
		t.Fatal("New(\"\") should default to compose")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/`
Expected: FAIL — package/symbols undefined.

- [ ] **Step 3: Implement**

`internal/runner/runner.go`:
```go
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
```

> This won't compile until Tasks 3 & 4 add `composeRunner`/`tiltRunner`. That's expected — implement 3 and 4 next, then run tests.

- [ ] **Step 4: Commit (after Tasks 3 & 4 compile)**

Deferred — commit together with Task 4 once the package builds.

---

### Task 3: `composeRunner` + `buildComposeArgs`

**Files:**
- Create: `internal/runner/compose.go`
- Test: `internal/runner/compose_test.go`

- [ ] **Step 1: Write the failing test**

`internal/runner/compose_test.go`:
```go
package runner

import (
	"strings"
	"testing"
)

func TestBuildComposeArgs(t *testing.T) {
	got := buildComposeArgs("remind", "/p/docker-compose.yml", "/h/.lane/overrides/remind.override.yml", false)
	want := "compose -p remind -f /p/docker-compose.yml -f /h/.lane/overrides/remind.override.yml up -d"
	if strings.Join(got, " ") != want {
		t.Fatalf("got %q, want %q", strings.Join(got, " "), want)
	}
}

func TestBuildComposeArgs_Build(t *testing.T) {
	got := buildComposeArgs("x", "a.yml", "b.yml", true)
	if got[len(got)-1] != "--build" {
		t.Fatalf("expected --build last, got %v", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/runner/ -run TestBuildComposeArgs`
Expected: FAIL — `buildComposeArgs` undefined.

- [ ] **Step 3: Implement**

`internal/runner/compose.go`:
```go
package runner

import (
	"fmt"
	"os"
	"os/exec"
)

type composeRunner struct{}

func (composeRunner) Name() string { return "compose" }

// buildComposeArgs builds the `docker <args>` for bringing a stack up detached.
func buildComposeArgs(slug, composePath, overridePath string, build bool) []string {
	args := []string{"compose", "-p", slug, "-f", composePath, "-f", overridePath, "up", "-d"}
	if build {
		args = append(args, "--build")
	}
	return args
}

func (composeRunner) DryRunLines(s RunSpec) string {
	return fmt.Sprintf("# runner: compose\n# command: docker %v\n",
		buildComposeArgs(s.Slug, s.ComposePath, s.OverridePath, s.Build))
}

func (composeRunner) Up(s RunSpec) error {
	printURLs(s)
	c := exec.Command("docker", buildComposeArgs(s.Slug, s.ComposePath, s.OverridePath, s.Build)...)
	c.Dir = s.Dir
	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Run(); err != nil {
		return err
	}
	fmt.Printf("up (detached). logs: lane logs --slug %s\n", s.Slug)
	return nil
}
```

- [ ] **Step 4: Do not run tests yet**

The package still won't compile — `runner.New()` (Task 2) references
`tiltRunner`, added in Task 4. Write `compose.go` now; the whole `internal/runner`
package is compiled and tested in Task 4 Step 2.

---

### Task 4: `tiltRunner` (extract from cmd/up.go) + compile + commit package

**Files:**
- Create: `internal/runner/tilt.go`

- [ ] **Step 1: Implement** (moves the Tilt exec/pidfile/dynamic-route logic out of `cmd/up.go`)

`internal/runner/tilt.go`:
```go
package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dheeraj-nalapat/lane/internal/paths"
	"github.com/dheeraj-nalapat/lane/internal/tiltx"
)

type tiltRunner struct{}

func (tiltRunner) Name() string { return "tilt" }

func (tiltRunner) DryRunLines(s RunSpec) string {
	dyn, _ := tiltx.RenderDynamicRoute(s.Slug, s.TiltPort)
	return fmt.Sprintf("# runner: tilt\n# tilt port: %d\n# tilt dynamic (%s):\n%s\n# command: tilt %v\n# env adds: COMPOSE_PROJECT_NAME, LANE, LANE_SLUG, LANE_COMPOSE_OVERRIDE\n",
		s.TiltPort, s.DynamicPath, dyn, tiltx.UpArgs(s.TiltPort))
}

func (tiltRunner) Up(s RunSpec) error {
	dyn, err := tiltx.RenderDynamicRoute(s.Slug, s.TiltPort)
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.DynamicPath, dyn, 0o644); err != nil {
		return err
	}

	printURLs(s)
	fmt.Printf("  → http://tilt-%s.localhost  (Tilt UI)\n", s.Slug)

	tcmd := exec.Command("tilt", tiltx.UpArgs(s.TiltPort)...)
	tcmd.Dir = s.Dir
	tcmd.Env = s.Env

	if s.Detach {
		logf, err := os.Create(filepath.Join(paths.Run(), s.Slug+".log"))
		if err != nil {
			return err
		}
		tcmd.Stdout, tcmd.Stderr = logf, logf
		if err := tcmd.Start(); err != nil {
			return err
		}
		_ = os.WriteFile(filepath.Join(paths.Run(), s.Slug+".pid"),
			[]byte(fmt.Sprint(tcmd.Process.Pid)), 0o644)
		fmt.Printf("detached (pid %d). logs: lane logs --slug %s\n", tcmd.Process.Pid, s.Slug)
		return nil
	}
	tcmd.Stdout, tcmd.Stderr, tcmd.Stdin = os.Stdout, os.Stderr, os.Stdin
	return tcmd.Run()
}
```

- [ ] **Step 2: Build the package + run its tests**

Run: `go build ./internal/runner/ && go test ./internal/runner/`
Expected: builds; `TestSelect`, `TestNew`, `TestBuildComposeArgs*` PASS.

(`cmd/` will not build yet — `cmd/up.go` is rewired in Task 5/6. That's fine for this package-scoped check.)

- [ ] **Step 3: Commit the runner package**

```bash
git add internal/runner/
git commit -m "feat(runner): Runner interface, Select, compose + tilt runners"
```

---

### Task 5: Override — omit Tilt-port label when not Tilt

**Files:**
- Modify: `internal/override/override.go`
- Test: `internal/override/override_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/override/override_test.go`:
```go
func TestGenerate_NoTiltPortLabelWhenZero(t *testing.T) {
	out, err := Generate(Spec{
		Slug: "demo", ProjectPath: "/p", Network: "lane",
		Services: []string{"web"}, TiltPort: 0,
		Routes: []Route{{Service: "web", Port: 80, Hostname: "demo.localhost"}},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if strings.Contains(string(out), "lane.tilt.port") {
		t.Fatalf("compose stack (TiltPort 0) must not emit lane.tilt.port:\n%s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/override/ -run TestGenerate_NoTiltPortLabelWhenZero`
Expected: FAIL — label `lane.tilt.port=0` present.

- [ ] **Step 3: Implement**

In `internal/override/override.go`, replace the `idLabels` initialization:
```go
	idLabels := []string{
		"lane.managed=true",
		"lane.slug=" + s.Slug,
		"lane.project.path=" + s.ProjectPath,
		fmt.Sprintf("lane.tilt.port=%d", s.TiltPort),
	}
```
with:
```go
	idLabels := []string{
		"lane.managed=true",
		"lane.slug=" + s.Slug,
		"lane.project.path=" + s.ProjectPath,
	}
	if s.TiltPort > 0 {
		idLabels = append(idLabels, fmt.Sprintf("lane.tilt.port=%d", s.TiltPort))
	}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/override/`
Expected: PASS (existing `TestGenerate` uses TiltPort 10377 so still emits the label; new test passes).

- [ ] **Step 5: Commit**

```bash
git add internal/override/
git commit -m "feat(override): omit lane.tilt.port label for compose stacks"
```

---

### Task 6: Rewire `cmd/up.go` to select + dispatch; add `--build`

**Files:**
- Modify: `cmd/up.go`

- [ ] **Step 1: Replace `cmd/up.go` entirely**

```go
package cmd

import (
	"fmt"
	"os"
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
	"github.com/dheeraj-nalapat/lane/internal/runner"
	"github.com/dheeraj-nalapat/lane/internal/slug"
	"github.com/spf13/cobra"
)

var (
	flagDetach bool
	flagBuild  bool
)

var upCmd = &cobra.Command{
	Use:   "up [path]",
	Short: "Bring a stack up behind the lane proxy",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runUp,
}

func init() {
	upCmd.Flags().BoolVarP(&flagDetach, "detach", "d", false, "run Tilt in the background (no-op for the compose runner)")
	upCmd.Flags().BoolVar(&flagBuild, "build", false, "force image rebuild (compose runner)")
	root.AddCommand(upCmd)
}

func tiltfileExists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "Tiltfile"))
	return err == nil
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

	if claimed, ok := dockerx.SlugOwner(sl); ok && claimed != dir {
		return fmt.Errorf("slug %q already in use by stack at %s; pass --slug to disambiguate", sl, claimed)
	}

	composePath := filepath.Join(dir, m.ComposeFile)
	svcs, err := compose.ServiceNames(composePath)
	if err != nil {
		return err
	}

	runnerName := runner.Select(m.Runner, tiltfileExists(dir))
	if m.Runner == "tilt" && !tiltfileExists(dir) {
		fmt.Fprintln(os.Stderr, "lane: warning: runner=tilt but no Tiltfile found in", dir)
	}

	tiltPort := 0
	if runnerName == "tilt" {
		if tiltPort, err = ports.Free(); err != nil {
			return err
		}
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

	env := append(os.Environ(),
		"COMPOSE_PROJECT_NAME="+sl,
		"LANE=1",
		"LANE_SLUG="+sl,
		"LANE_COMPOSE_OVERRIDE="+overridePath,
	)
	if m.APITarget != "" {
		env = append(env, "LANE_API_TARGET=http://"+m.APITarget)
	}

	spec := runner.RunSpec{
		Slug: sl, Dir: dir, ComposePath: composePath, OverridePath: overridePath,
		Routes: routes, Detach: flagDetach, Build: flagBuild,
		TiltPort: tiltPort, DynamicPath: dynamicPath, Env: env,
	}
	r := runner.New(runnerName)

	if flagDryRun {
		fmt.Printf("# slug: %s\n# override (%s):\n%s\n%s", sl, overridePath, body, r.DryRunLines(spec))
		return nil
	}

	if err := os.WriteFile(overridePath, body, 0o644); err != nil {
		return err
	}
	if err := proxy.Ensure(); err != nil {
		return err
	}
	return r.Up(spec)
}

func projectDir(args []string) (string, error) {
	if len(args) == 1 {
		return filepath.Abs(args[0])
	}
	return os.Getwd()
}
```

(Note: `printURLs` moved to the runner package, so it is removed from `cmd/up.go`. The `tiltx` import is gone from this file.)

- [ ] **Step 2: Build + run full unit suite**

Run: `go build ./... && go test ./... && go vet ./...`
Expected: builds; all tests PASS; vet clean.

- [ ] **Step 3: Dry-run sanity (no services started)**

```bash
# In a Tilt project (e.g. ReMind): expect "# runner: tilt"
cd /home/dheerajnalapat/project/ReMind && go run /home/dheerajnalapat/project/lane up --dry-run | grep -E '# runner|# slug'
```
Expected: `# slug: remind` and `# runner: tilt`.

- [ ] **Step 4: Commit**

```bash
cd /home/dheerajnalapat/project/lane
git add cmd/up.go
git commit -m "feat(up): select runner (tilt|compose), add --build, dispatch via runner"
```

---

### Task 7: `view`/`ls` guard on Tilt port

**Files:**
- Modify: `internal/ui/view.go`, `cmd/ls.go`
- Test: `internal/ui/view_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/ui/view_test.go`:
```go
func TestRender_ComposeStackHidesTiltRow(t *testing.T) {
	out := Render(
		[]stack.Stack{{Slug: "demo", URL: "http://demo.localhost", TiltPort: 0, ProjectPath: "/p", Running: true}},
		nil,
	)
	if strings.Contains(out, "tilt →") {
		t.Fatalf("compose stack (TiltPort 0) must not show a tilt row:\n%s", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ui/ -run TestRender_ComposeStackHidesTiltRow`
Expected: FAIL — tilt row rendered with `:0`.

- [ ] **Step 3: Implement the guards**

In `internal/ui/view.go`, wrap the Tilt line (currently unconditional) in a check:
```go
		if s.TiltPort > 0 {
			b.WriteString(fmt.Sprintf("  %s\n", dimStyle.Render(fmt.Sprintf("tilt → http://tilt-%s.localhost (:%d)", s.Slug, s.TiltPort))))
		}
```

In `cmd/ls.go`, change the row printf to show `-` when there's no Tilt port. Replace:
```go
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n", s.Slug, s.URL, s.TiltPort, state, s.ProjectPath)
```
with:
```go
			tilt := "-"
			if s.TiltPort > 0 {
				tilt = fmt.Sprintf("%d", s.TiltPort)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.Slug, s.URL, tilt, state, s.ProjectPath)
```

- [ ] **Step 4: Run tests + build**

Run: `go test ./internal/ui/ ./cmd/ && go build ./...`
Expected: PASS, builds.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/view.go cmd/ls.go
git commit -m "feat(view,ls): hide Tilt row/port for compose stacks"
```

---

### Task 8: Docs — Tiltfile shim is Tilt-only; compose path is zero-config

**Files:**
- Modify: `README.md`, `docs/onboarding-remind.md`

- [ ] **Step 1: Update README onboarding**

In `README.md`, in the "Onboarding a project" section, change the heading/intro of the Tiltfile-shim step to make clear it is **only** for Tilt projects, and add a short compose note. Replace the paragraph that introduces the Tiltfile shim ("### 2. Tiltfile shim (in the `--docker` branch)") intro line:
```
### 2. Tiltfile shim (in the `--docker` branch)
```
with:
```
### 2. Tiltfile shim — **Tilt projects only**

If your project has **no Tiltfile**, skip this entirely: lane detects the
absence of a Tiltfile and runs `docker compose` directly (project name set via
`-p <slug>`), so a plain compose project needs **only** the `.lane.toml` above —
no Tiltfile, no shim. The steps below apply only when you use Tilt for
live-reload + the Tilt dashboard.
```

Also add a row to the Commands table note for `up`: mention `--build` (compose
runner: force rebuild) and that `-d` is a no-op for the compose runner (it is
always detached).

- [ ] **Step 2: Update onboarding-remind.md contract note**

In `docs/onboarding-remind.md`, under "Tiltfile contract", prepend a sentence:
```
> These requirements apply **only to Tilt projects**. A project with no Tiltfile
> is driven by `docker compose` directly and needs none of this — just a
> `.lane.toml`.
```

- [ ] **Step 3: Commit**

```bash
git add README.md docs/onboarding-remind.md
git commit -m "docs: compose runner is zero-config; Tiltfile shim is Tilt-only"
```

---

### Task 9: Live smoke — Tilt-less compose project end-to-end (manual)

**No code. Acceptance test. Requires Docker.**

- [ ] **Step 1: Build + create a Tilt-less project**

```bash
go build -o ~/.local/bin/lane /home/dheerajnalapat/project/lane
mkdir -p /tmp/lane-nocompose
cat > /tmp/lane-nocompose/docker-compose.yml <<'EOF'
services:
  web:
    image: traefik/whoami
    container_name: nc-web      # lane must !reset this
    ports: ["8088:80"]          # lane must strip this
EOF
cat > /tmp/lane-nocompose/.lane.toml <<'EOF'
name = "nocompose"
compose_file = "docker-compose.yml"
[[routes]]
service = "web"
port = 80
EOF
# NOTE: deliberately NO Tiltfile.
```

- [ ] **Step 2: Bring it up and verify (no Tiltfile → compose runner)**

```bash
lane proxy up
cd /tmp/lane-nocompose && lane up --dry-run | grep '# runner'   # expect: # runner: compose
lane up
lane ls                                                          # TILT column shows "-"
curl -s http://nocompose.localhost/ | grep -E '^(Host|Hostname):'
docker ps --filter label=lane.slug=nocompose --format '{{.Names}} {{.Ports}}'  # name nocompose-web-1, no 8088 published
```
Expected: `# runner: compose`; `http://nocompose.localhost` returns whoami with `Host: nocompose.localhost`; container is `nocompose-web-1` with only `80/tcp` (8088 stripped); `lane ls` shows `-` in the TILT column.

- [ ] **Step 3: Tear down + clean**

```bash
cd /tmp/lane-nocompose && lane down
lane proxy down && docker network rm lane 2>/dev/null
rm -rf /tmp/lane-nocompose
```
Expected: `lane down` removes the stack (it already uses `docker compose down`); `lane ls` empty.

---

## Final verification

- [ ] `go test ./...` — all pass (manifest, runner, override, ui, plus existing).
- [ ] `go vet ./...` — clean.
- [ ] `lane up --dry-run` in a Tilt project prints `# runner: tilt`; in a Tilt-less project prints `# runner: compose`.
- [ ] Task 9 live smoke passed: a no-Tiltfile compose project runs behind the proxy with zero shim and tears down cleanly.
- [ ] Existing Tilt project (ReMind) still selects the tilt runner and behaves exactly as before (backward compatible).
