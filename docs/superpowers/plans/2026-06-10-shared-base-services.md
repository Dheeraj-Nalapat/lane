# Shared Base Services (Base-Borrowing) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `lane up <services...> --base` runs the named services fresh and wires every other (borrowed) service to a running base stack of the same project via per-service `docker network connect`, so a worktree reuses the base's unchanged services instead of booting its own.

**Architecture:** A base is a normal full `lane up` from the main checkout. A worktree brings up only its named services with compose `--no-deps`, then lane connects each borrowed base container into the worktree's compose network (`<slug>_default`) with the service-name alias. Fresh services resolve each other locally; borrowed names resolve to the base. Builds on sub-project A (fresh services are auto-routed at `<slug>-<service>.localhost`).

**Tech Stack:** Go 1.22, cobra, Docker Compose v2, `docker network connect/disconnect/inspect`.

**Spec:** `docs/superpowers/specs/2026-06-10-shared-base-services-design.md`
**Branch:** `feat/shared-services` (stacked on `feat/selective-bring-up` / sub-project A).

**Conventions (same as A):** table-driven `_test.go` alongside sources; `go test ./...`, `go vet ./...`, `gofmt -l .` (empty) before each commit; one commit per task; no `Co-Authored-By` trailers.

---

## File map

| File | Responsibility | Change |
|---|---|---|
| `internal/override/override.go` | generate overlay | add `lane.project` label (new `Spec.Project`) |
| `cmd/up.go` | `lane up` | pass `Project`; `--base` flag + base-mode branch |
| `internal/stack/stack.go` | shared Stack model | add `Project` field |
| `internal/dockerx/dockerx.go` | query/mutate Docker | parse `lane.project`; `RunningContainers`, `NetworkConnect/Disconnect`, `ForeignContainers` |
| `internal/basex/basex.go` | **new** — base discovery + borrowed-set | new package |
| `internal/runner/runner.go`, `compose.go` | runner | `RunSpec.NoDeps` + `--no-deps` arg |
| `cmd/down.go` | teardown | disconnect foreign containers before `compose down` |
| `docs/guide/base-stacks.md` | **new** docs page | usage |

---

## Task 1: `lane.project` identity label

**Files:**
- Modify: `internal/override/override.go`, `cmd/up.go`
- Test: `internal/override/override_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/override/override_test.go`:

```go
func TestGenerate_ProjectLabel(t *testing.T) {
	out, err := Generate(Spec{
		Slug: "webapp-featx", Project: "webapp", ProjectPath: "/p", Network: "lane",
		Services: []string{"web"},
		Routes:   []Route{{Service: "web", Port: 80, Hostname: "webapp-featx.localhost"}},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.Contains(string(out), "lane.project=webapp") {
		t.Fatalf("missing lane.project label:\n%s", out)
	}
}
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/override/`
Expected: FAIL — `unknown field Project in struct literal`.

- [ ] **Step 3: Add `Project` to `Spec` and emit the label**

In `internal/override/override.go`, add the field to `Spec` (after `Slug`):

```go
type Spec struct {
	Slug          string
	Project       string // manifest name, shared across a project's stacks
	ProjectPath   string
```

In `Generate`, extend `idLabels` (currently at `override.go:52`):

```go
	idLabels := []string{
		"lane.managed=true",
		"lane.slug=" + s.Slug,
		"lane.project=" + s.Project,
		"lane.project.path=" + s.ProjectPath,
	}
```

- [ ] **Step 4: Pass `Project` from `cmd/up.go`**

In `cmd/up.go`, in the `override.Generate(override.Spec{...})` call, add `Project: m.Name`:

```go
	body, err := override.Generate(override.Spec{
		Slug: sl, Project: m.Name, ProjectPath: dir, Network: proxy.Network,
		Services: svcs, Routes: routes, TiltPort: tiltPort, TLS: tlsOn,
		BuiltServices: built,
	})
```

- [ ] **Step 5: Run tests + build, verify pass**

Run: `go test ./internal/override/ && go build ./...`
Expected: PASS + clean build.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/override/ cmd/up.go
go vet ./internal/override/ ./cmd/
git add internal/override/ cmd/up.go
git commit -m "feat(override): add lane.project identity label"
```

---

## Task 2: Surface `Project` on the Stack model

**Files:**
- Modify: `internal/stack/stack.go`, `internal/dockerx/dockerx.go`
- Test: `internal/dockerx/dockerx_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/dockerx/dockerx_test.go`:

```go
func TestParsePS_Project(t *testing.T) {
	lines := `{"Names":"webapp-ui-1","Labels":"lane.managed=true,lane.slug=webapp,lane.project=webapp,lane.project.path=/p","State":"running"}`
	stacks := parsePS([]byte(lines))
	if len(stacks) != 1 {
		t.Fatalf("got %d stacks, want 1", len(stacks))
	}
	if stacks[0].Project != "webapp" {
		t.Fatalf("Project = %q, want webapp", stacks[0].Project)
	}
}
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/dockerx/`
Expected: FAIL — `s.Project undefined` (build error).

- [ ] **Step 3: Add the field + parse it**

In `internal/stack/stack.go`, add to `Stack`:

```go
type Stack struct {
	Slug        string   `json:"slug"`
	Project     string   `json:"project"`
	URL         string   `json:"url"`
```

In `internal/dockerx/dockerx.go`, inside `parsePS`, where the stack is created/updated, set `Project` from the label. Change the `s == nil` block:

```go
		s := bySlug[sl]
		if s == nil {
			s = &stack.Stack{Slug: sl, Project: lbl["lane.project"], ProjectPath: lbl["lane.project.path"]}
			bySlug[sl] = s
		}
```

- [ ] **Step 4: Run tests + build, verify pass**

Run: `go test ./internal/dockerx/ ./internal/stack/ ./internal/ui/ && go build ./...`
Expected: PASS + clean build (the `ui` package marshals `Stack`; new field is additive).

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/stack/ internal/dockerx/
go vet ./internal/stack/ ./internal/dockerx/
git add internal/stack/ internal/dockerx/
git commit -m "feat(stack,dockerx): surface project name on Stack"
```

---

## Task 3: dockerx — container lookup + network ops

**Files:**
- Modify: `internal/dockerx/dockerx.go`
- Test: `internal/dockerx/dockerx_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/dockerx/dockerx_test.go`:

```go
func TestParseContainers(t *testing.T) {
	out := []byte(`{"Names":"webapp-db-1","Labels":"com.docker.compose.service=db,com.docker.compose.project=webapp","State":"running"}
{"Names":"webapp-api-1","Labels":"com.docker.compose.project=webapp,com.docker.compose.service=api","State":"running"}
{"Names":"webapp-old-1","Labels":"com.docker.compose.service=old,com.docker.compose.project=webapp","State":"exited"}
`)
	got := parseContainers(out)
	byService := map[string]string{}
	for _, c := range got {
		byService[c.Service] = c.Name
	}
	if byService["db"] != "webapp-db-1" || byService["api"] != "webapp-api-1" {
		t.Fatalf("unexpected containers: %v", got)
	}
	if _, ok := byService["old"]; ok {
		t.Fatal("exited container must be excluded")
	}
}

func TestParseForeignContainers(t *testing.T) {
	// `docker network inspect <net> --format '{{json .Containers}}'` output.
	out := []byte(`{
		"abc": {"Name": "webapp-featx-api-1"},
		"def": {"Name": "webapp-db-1"},
		"ghi": {"Name": "webapp-auth-1"}
	}`)
	got := parseForeignContainers(out, "webapp-featx")
	want := []string{"webapp-auth-1", "webapp-db-1"} // sorted; the featx one is ours
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestNetworkArgs(t *testing.T) {
	c := connectArgs("webapp-featx_default", "webapp-db-1", "db")
	if strings.Join(c, " ") != "network connect --alias db webapp-featx_default webapp-db-1" {
		t.Fatalf("connectArgs = %v", c)
	}
	d := disconnectArgs("webapp-featx_default", "webapp-db-1")
	if strings.Join(d, " ") != "network disconnect webapp-featx_default webapp-db-1" {
		t.Fatalf("disconnectArgs = %v", d)
	}
}
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/dockerx/`
Expected: FAIL — `parseContainers`, `parseForeignContainers`, `connectArgs`, `disconnectArgs` undefined.

- [ ] **Step 3: Implement the helpers**

Append to `internal/dockerx/dockerx.go` (add `"sort"` to imports):

```go
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
```

- [ ] **Step 4: Run tests, verify pass**

Run: `go test ./internal/dockerx/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/dockerx/
go vet ./internal/dockerx/
git add internal/dockerx/
git commit -m "feat(dockerx): running-container lookup + network connect/disconnect/inspect"
```

---

## Task 4: `basex` — base discovery + borrowed set

**Files:**
- Create: `internal/basex/basex.go`, `internal/basex/basex_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/basex/basex_test.go`:

```go
package basex

import (
	"testing"

	"github.com/dheeraj-nalapat/lane/internal/stack"
)

func stacks() []stack.Stack {
	return []stack.Stack{
		{Slug: "webapp", Project: "webapp", Running: true},
		{Slug: "webapp-featx", Project: "webapp", Running: true},
		{Slug: "other", Project: "other", Running: true},
	}
}

func TestFindBase_Canonical(t *testing.T) {
	got, err := FindBase(stacks(), "webapp", "webapp-featx")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if got != "webapp" {
		t.Fatalf("base = %q, want webapp", got)
	}
}

func TestFindBase_IsBase(t *testing.T) {
	if _, err := FindBase(stacks(), "webapp", "webapp"); err == nil {
		t.Fatal("expected error when run from the base itself")
	}
}

func TestFindBase_None(t *testing.T) {
	only := []stack.Stack{{Slug: "x-featx", Project: "x", Running: false}}
	if _, err := FindBase(only, "x", "x-featx"); err == nil {
		t.Fatal("expected error when no running base")
	}
}

func TestFindBase_Multiple(t *testing.T) {
	ss := []stack.Stack{
		{Slug: "webapp-a", Project: "webapp", Running: true},
		{Slug: "webapp-b", Project: "webapp", Running: true},
	}
	if _, err := FindBase(ss, "webapp", "webapp-featx"); err == nil {
		t.Fatal("expected error for multiple candidates (no canonical)")
	}
}

func TestBorrowed(t *testing.T) {
	got := Borrowed([]string{"web", "api", "db", "auth"}, []string{"api"})
	want := []string{"auth", "db", "web"} // sorted, minus fresh
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/basex/`
Expected: FAIL — package `basex` does not exist.

- [ ] **Step 3: Implement `basex`**

Create `internal/basex/basex.go`:

```go
// Package basex resolves the base stack a worktree borrows from and computes
// which services are borrowed vs run fresh.
package basex

import (
	"fmt"
	"sort"

	"github.com/dheeraj-nalapat/lane/internal/stack"
)

// FindBase returns the slug of the running base stack for project, excluding the
// caller's own slug. Prefers the canonical base (slug == project). Errors when
// the caller is the base, when none is running, or when multiple non-canonical
// candidates exist.
func FindBase(stacks []stack.Stack, project, ownSlug string) (string, error) {
	if ownSlug == project {
		return "", fmt.Errorf("this is the base stack; --base is for worktrees")
	}
	var cands []string
	for _, s := range stacks {
		if s.Project != project || s.Slug == ownSlug || !s.Running {
			continue
		}
		if s.Slug == project {
			return s.Slug, nil // canonical base wins
		}
		cands = append(cands, s.Slug)
	}
	sort.Strings(cands)
	switch len(cands) {
	case 0:
		return "", fmt.Errorf("no running base for %q; start it from the main checkout with `lane up`", project)
	case 1:
		return cands[0], nil
	default:
		return "", fmt.Errorf("multiple candidate base stacks for %q: %v (explicit base selection is not yet supported)", project, cands)
	}
}

// Borrowed returns the base services not in the fresh set, sorted.
func Borrowed(baseServices, fresh []string) []string {
	fset := map[string]bool{}
	for _, f := range fresh {
		fset[f] = true
	}
	var b []string
	for _, s := range baseServices {
		if !fset[s] {
			b = append(b, s)
		}
	}
	sort.Strings(b)
	return b
}
```

- [ ] **Step 4: Run tests, verify pass**

Run: `go test ./internal/basex/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w internal/basex/
go vet ./internal/basex/
git add internal/basex/
git commit -m "feat(basex): base-stack discovery and borrowed-set computation"
```

---

## Task 5: Runner — `--no-deps`

**Files:**
- Modify: `internal/runner/runner.go`, `internal/runner/compose.go`
- Test: `internal/runner/compose_test.go`

- [ ] **Step 1: Update the existing compose-args tests + add a no-deps test**

In `internal/runner/compose_test.go`, the existing three calls to `buildComposeArgs` gain a `noDeps` parameter (the 5th positional, after `build`). Replace the file with:

```go
package runner

import (
	"strings"
	"testing"
)

func TestBuildComposeArgs(t *testing.T) {
	got := buildComposeArgs("remind", "/p/docker-compose.yml", "/h/.lane/overrides/remind.override.yml", false, false, nil, nil)
	want := "compose -p remind -f /p/docker-compose.yml -f /h/.lane/overrides/remind.override.yml up -d"
	if strings.Join(got, " ") != want {
		t.Fatalf("got %q, want %q", strings.Join(got, " "), want)
	}
}

func TestBuildComposeArgs_Build(t *testing.T) {
	got := buildComposeArgs("x", "a.yml", "b.yml", true, false, nil, nil)
	if got[len(got)-1] != "--build" {
		t.Fatalf("expected --build last, got %v", got)
	}
}

func TestBuildComposeArgs_ProfilesAndServices(t *testing.T) {
	got := buildComposeArgs("webapp", "compose.yml", "ovr.yml", true, false,
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

func TestBuildComposeArgs_NoDeps(t *testing.T) {
	got := buildComposeArgs("webapp", "compose.yml", "ovr.yml", false, true, nil, []string{"api"})
	want := []string{"compose", "-p", "webapp", "-f", "compose.yml", "-f", "ovr.yml", "up", "-d", "--no-deps", "api"}
	if len(got) != len(want) {
		t.Fatalf("len mismatch:\n got %v\nwant %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("arg %d = %q, want %q\nfull: %v", i, got[i], want[i], got)
		}
	}
}
```

- [ ] **Step 2: Run it, verify it fails**

Run: `go test ./internal/runner/`
Expected: FAIL — `buildComposeArgs` takes 6 args, calls pass 7/8.

- [ ] **Step 3: Add `NoDeps` to `RunSpec`**

In `internal/runner/runner.go`, add to `RunSpec` (after `Profiles`):

```go
	Services     []string // subset to bring up; empty = all
	Profiles     []string // compose profiles to activate
	NoDeps       bool     // compose --no-deps (base mode: borrow deps instead)
```

- [ ] **Step 4: Thread `noDeps` through `buildComposeArgs`**

In `internal/runner/compose.go`, replace `buildComposeArgs` and its two callers:

```go
// buildComposeArgs builds the `docker <args>` for bringing a stack up detached.
// Global flags (--profile, -p, -f) must precede `up`; service names follow it.
func buildComposeArgs(slug, composePath, overridePath string, build, noDeps bool, profiles, services []string) []string {
	args := []string{"compose"}
	for _, p := range profiles {
		args = append(args, "--profile", p)
	}
	args = append(args, "-p", slug, "-f", composePath, "-f", overridePath, "up", "-d")
	if noDeps {
		args = append(args, "--no-deps")
	}
	if build {
		args = append(args, "--build")
	}
	args = append(args, services...)
	return args
}

func (composeRunner) DryRunLines(s RunSpec) string {
	return fmt.Sprintf("# runner: compose\n# command: docker %v\n",
		buildComposeArgs(s.Slug, s.ComposePath, s.OverridePath, s.Build, s.NoDeps, s.Profiles, s.Services))
}

func (composeRunner) Up(s RunSpec) error {
	printURLs(s)
	c := exec.Command("docker", buildComposeArgs(s.Slug, s.ComposePath, s.OverridePath, s.Build, s.NoDeps, s.Profiles, s.Services)...)
```

- [ ] **Step 5: Run tests + build, verify pass**

Run: `go test ./internal/runner/ && go build ./...`
Expected: PASS + clean build.

- [ ] **Step 6: Commit**

```bash
gofmt -w internal/runner/
go vet ./internal/runner/
git add internal/runner/
git commit -m "feat(runner): support compose --no-deps via RunSpec.NoDeps"
```

---

## Task 6: `lane up --base` wiring

**Files:**
- Modify: `cmd/up.go`

- [ ] **Step 1: Add the `--base` flag**

In `cmd/up.go`, add to the `var (...)` block:

```go
	flagBase bool
```

In `init()`, add:

```go
	upCmd.Flags().BoolVar(&flagBase, "base", false, "run named services fresh and borrow the rest from a running base stack")
```

Add imports to `cmd/up.go`: `"github.com/dheeraj-nalapat/lane/internal/basex"`. (`dockerx`, `strings` already imported from A.)

- [ ] **Step 2: Extend the JSON result struct**

In `cmd/up.go`, add base fields to `upResult`:

```go
type upResult struct {
	Slug     string   `json:"slug"`
	Runner   string   `json:"runner"`
	TLS      bool     `json:"tls"`
	Base     string   `json:"base,omitempty"`
	Fresh    []string `json:"fresh,omitempty"`
	Borrowed []string `json:"borrowed,omitempty"`
	TiltURL  string   `json:"tiltUrl,omitempty"`
	URLs     []upURL  `json:"urls"`
}
```

- [ ] **Step 3: Validate base mode + discover the base (before bring-up)**

In `runUp`, immediately after `runnerName := runner.Select(...)` and the `selection :=` line (from A), insert base-mode validation and discovery:

```go
	baseSlug := ""
	if flagBase {
		if runnerName != "compose" {
			return fmt.Errorf("base mode requires the compose runner")
		}
		if len(args) == 0 {
			return fmt.Errorf("base mode needs at least one service to run fresh, e.g. `lane up api --base`")
		}
		if !flagDryRun {
			stacks, err := dockerx.List()
			if err != nil {
				return err
			}
			baseSlug, err = basex.FindBase(stacks, m.Name, sl)
			if err != nil {
				return err
			}
		}
	}
```

- [ ] **Step 4: Set `NoDeps` on the run spec**

In the `spec := runner.RunSpec{...}` literal, add `NoDeps: flagBase`:

```go
		Quiet:    flagJSON,
		Services: args,
		Profiles: flagProfiles,
		NoDeps:   flagBase,
	}
```

- [ ] **Step 5: Wire borrowed services after bring-up**

In `runUp`, immediately after `if err := r.Up(spec); err != nil { return err }`, insert the wiring block:

```go
	var borrowed []string
	if flagBase {
		baseContainers, err := dockerx.RunningContainers(baseSlug)
		if err != nil {
			return err
		}
		nameBySvc := map[string]string{}
		var baseSvcs []string
		for _, c := range baseContainers {
			nameBySvc[c.Service] = c.Name
			baseSvcs = append(baseSvcs, c.Service)
		}
		borrowed = basex.Borrowed(baseSvcs, args)
		net := sl + "_default"
		for _, svc := range borrowed {
			cn := nameBySvc[svc]
			if cn == "" {
				continue
			}
			if err := dockerx.NetworkConnect(net, cn, svc); err != nil {
				fmt.Fprintf(os.Stderr, "lane: warning: wiring %q from base %q: %v\n", svc, baseSlug, err)
			}
		}
		if !flagJSON {
			fmt.Fprintf(os.Stderr, "lane: borrowing from %s: %s\n", baseSlug, strings.Join(borrowed, ", "))
		}
	}
```

- [ ] **Step 6: Include base info in `--json`**

Replace the `if flagJSON { return printJSON(buildUpResult(...)) }` inside the wait/json tail with a version that sets base fields:

```go
		if flagJSON {
			res := buildUpResult(sl, runnerName, tlsOn, routes, tiltPort, running)
			if flagBase {
				res.Base = baseSlug
				res.Fresh = args
				res.Borrowed = borrowed
			}
			return printJSON(res)
		}
```

- [ ] **Step 7: Build + tests**

Run: `go build ./... && go test ./cmd/`
Expected: clean build; PASS.

- [ ] **Step 8: Dry-run smoke check (no Docker needed)**

```bash
go build -o /tmp/lane .
mkdir -p /tmp/lane-base-dry
printf 'services:\n  api:\n    image: x\n    expose: ["8000"]\n  db:\n    image: y\n    expose: ["5432"]\n' > /tmp/lane-base-dry/docker-compose.yml
printf 'name = "basedry"\ncompose_file = "docker-compose.yml"\n' > /tmp/lane-base-dry/.lane.toml
/tmp/lane up api --base --dry-run -C /tmp/lane-base-dry
rm -rf /tmp/lane-base-dry
```
Expected: the compose command line contains `up -d --no-deps api` (dry-run skips base discovery and wiring).

- [ ] **Step 9: Commit**

```bash
gofmt -w cmd/up.go
go vet ./...
git add cmd/up.go
git commit -m "feat(up): --base mode — run named services fresh, borrow the rest"
```

---

## Task 7: `lane down` — disconnect borrowed base containers

**Files:**
- Modify: `cmd/down.go`

- [ ] **Step 1: Disconnect foreign containers before `compose down`**

In `cmd/down.go`, the `runDown` function builds `dcArgs` then runs `docker compose down` (around `down.go:60`). Immediately **before** the `dc := exec.Command("docker", dcArgs...)` line, insert:

```go
	// Base mode connects borrowed base containers into this stack's network.
	// Disconnect them first, else compose can't remove the network ("active
	// endpoints"). Base containers are only disconnected, never stopped.
	net := sl + "_default"
	if foreign, err := dockerx.ForeignContainers(net, sl); err == nil {
		for _, c := range foreign {
			_ = dockerx.NetworkDisconnect(net, c)
		}
	}
```

Add the import `"github.com/dheeraj-nalapat/lane/internal/dockerx"` to `cmd/down.go`.

- [ ] **Step 2: Build + tests**

Run: `go build ./... && go test ./cmd/`
Expected: clean build; PASS. (`ForeignContainers` on a non-existent network returns an error, which we ignore — non-base downs are unaffected.)

- [ ] **Step 3: Commit**

```bash
gofmt -w cmd/down.go
go vet ./cmd/
git add cmd/down.go
git commit -m "fix(down): disconnect borrowed base containers before compose down"
```

---

## Task 8: Docs — "Borrowing from a base stack"

**Files:**
- Create: `docs/guide/base-stacks.md`
- Modify: `mkdocs.yml`

- [ ] **Step 1: Write the guide page**

Create `docs/guide/base-stacks.md`:

```markdown
# Borrowing from a base stack

When only a few services changed, run just those fresh in a worktree and borrow
everything else from a running **base** stack — instead of every worktree booting
the whole app.

## Set up the base

From your main checkout, bring the full stack up normally:

    lane up            # slug = your project name, e.g. "webapp"

## Borrow from it in a worktree

In a worktree, name the services you changed and add `--base`:

    lane up api --base          # api fresh; db, auth, web borrowed from webapp
    lane up web api --base      # web + api fresh; the rest borrowed

lane finds the base by project name (the `name` in `.lane.toml`), runs your named
services without their dependencies, and wires the rest to the base's containers.
Your fresh services are reachable as usual (see *Selecting & reaching services*),
e.g. `http://webapp-featx-api.localhost`.

## Rules and tips

- A service you **name** runs fresh; everything else is **borrowed**. To run a
  dependency fresh too (e.g. a schema change), name it: `lane up api db --base`.
- Borrowed services are the base's real containers — a borrowed `db` is shared
  data. Name it to get a private one.
- `lane up ... --base --json` reports `base`, `fresh`, and `borrowed`.
- `lane down` in the worktree disconnects the borrowed containers; the base keeps
  running.

## Limitations (v1)

- Compose runner only (Tilt errors clearly).
- Same project only (a worktree borrows from another stack of the same project).
- A freshly-started service may briefly start before its borrowed dependency is
  resolvable; apps that retry their dependency connections handle this cleanly.
- Assumes the project uses compose's default network.
```

- [ ] **Step 2: Add to nav**

In `mkdocs.yml`, add under `nav:` after the `Selecting services` line:

```yaml
  - Base stacks: base-stacks.md
```

- [ ] **Step 3: Strict docs build**

Run: `. .venv-docs/bin/activate && mkdocs build --strict && rm -rf site`
Expected: builds with no warnings/errors.

- [ ] **Step 4: Commit**

```bash
git add docs/guide/base-stacks.md mkdocs.yml
git commit -m "docs: add Borrowing from a base stack guide"
```

---

## Task 9: End-to-end verification

**Files:** none (manual, against real Docker; must not disturb other stacks).

- [ ] **Step 1: Create a throwaway project**

```bash
mkdir -p /tmp/lane-base-e2e
cat > /tmp/lane-base-e2e/docker-compose.yml <<'YAML'
services:
  app:
    image: busybox
    command: ["sleep", "infinity"]
  web:
    image: traefik/whoami
    expose: ["80"]
  db:
    image: busybox
    command: ["sleep", "infinity"]
YAML
cat > /tmp/lane-base-e2e/.lane.toml <<'TOML'
name = "be2e"
compose_file = "docker-compose.yml"
TOML
go build -o /tmp/lane .
```

- [ ] **Step 2: Bring up the base (full stack)**

```bash
/tmp/lane up --wait --json -C /tmp/lane-base-e2e
docker ps --filter label=com.docker.compose.project=be2e --format '{{.Names}}'
```
Expected: base slug `be2e`; containers `be2e-app-1`, `be2e-web-1`, `be2e-db-1` running.

- [ ] **Step 3: Borrow from the base in a "worktree" (simulated via --slug)**

```bash
/tmp/lane up app --base --slug be2e-featx --json -C /tmp/lane-base-e2e
```
Expected JSON: `"base":"be2e"`, `"fresh":["app"]`, `"borrowed":["db","web"]`. Only `be2e-featx-app-1` is a new container.

- [ ] **Step 4: Verify wiring + DNS resolution**

```bash
echo "--- base db/web connected into worktree network? ---"
docker network inspect be2e-featx_default --format '{{json .Containers}}' | tr ',' '\n' | grep -E 'be2e-(db|web)'
echo "--- worktree app resolves borrowed names to base containers ---"
docker exec be2e-featx-app-1 nslookup db 2>&1 | tail -3
docker exec be2e-featx-app-1 nslookup web 2>&1 | tail -3
```
Expected: `be2e-db-1` and `be2e-web-1` appear in the worktree network; `nslookup db`/`nslookup web` resolve to an IP (the base containers).

- [ ] **Step 5: Tear down the worktree; base stays up**

```bash
/tmp/lane down --slug be2e-featx -C /tmp/lane-base-e2e
echo "--- base still running? ---"
docker ps --filter label=com.docker.compose.project=be2e --format '{{.Names}}'
echo "--- worktree gone? ---"
docker ps -a --filter label=com.docker.compose.project=be2e-featx --format '{{.Names}}' || echo none
```
Expected: worktree torn down cleanly (no "active endpoints" error); `be2e-app-1`, `be2e-web-1`, `be2e-db-1` still running.

- [ ] **Step 6: Tear down the base + clean up**

```bash
/tmp/lane down -C /tmp/lane-base-e2e
docker ps -a --filter label=lane.project=be2e --format '{{.Names}}' || echo "all gone"
rm -rf /tmp/lane-base-e2e /tmp/lane
```
Expected: nothing left for project `be2e`.

---

## Self-review notes (author)

- **Spec coverage:** §2 model → Tasks 5/6; §3 no-deps → Task 5; §4 discovery + `lane.project` label → Tasks 1, 4 (+ Task 2 surfaces it); §5 wiring/lifecycle → Tasks 3 (ops), 6 (connect), 7 (disconnect); §6 json → Task 6; §7 structure → all; §8 testing → unit in 1–5, e2e in 9; §9 caveats → Task 8 docs. No gaps.
- **Type consistency:** `override.Spec.Project`, `stack.Stack.Project`, `dockerx.Container{Name,Service}`, `dockerx.RunningContainers/NetworkConnect(net,container,alias)/NetworkDisconnect(net,container)/ForeignContainers(net,ownSlug)`, `basex.FindBase([]stack.Stack,project,ownSlug)`, `basex.Borrowed(baseServices,fresh)`, `RunSpec.NoDeps`, `buildComposeArgs(...,build,noDeps,profiles,services)`, `upResult.Base/Fresh/Borrowed` — consistent across tasks.
- **Network name:** worktree compose network is `<slug>_default` (verified on this machine). Projects with only custom networks are unsupported in v1 (spec §9); `ForeignContainers` on a missing network errors and is ignored in `down`.
- **DNS timing:** wiring runs post-`up`; e2e uses busybox `sleep` services (no boot-time dep connection), so the timing caveat doesn't affect verification.
