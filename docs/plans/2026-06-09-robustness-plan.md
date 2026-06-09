# Robustness & UX (C1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden lane — clear preflight errors, idempotent `up` + `lane restart`, `down --volumes`, and per-slug image isolation — without changing the happy path.

**Architecture:** A new `internal/preflight` package centralizes Docker/Compose/port checks (shared with `doctor`). `up`/`proxy`/`tls` gate on it. `up` no-ops when a same-path stack already runs; `restart` = down+up. `down` gains `--volumes` and stops requiring the override file. The compose reader detects `build:` services so the override resets their `image:` (compose then names them by slug-project).

**Tech Stack:** Go 1.22, cobra, gopkg.in/yaml.v3 (existing). No new deps.

Spec: `docs/2026-06-09-robustness-design.md`.

---

## File Structure

```
internal/preflight/preflight.go       NEW — DockerRunning, ComposeOK(+composeOK), IsPortConflict
internal/preflight/preflight_test.go  NEW
internal/doctor/doctor.go             use preflight.ComposeOK; drop local composeOK/verRe
internal/doctor/doctor_test.go        DELETE (composeOK test moves to preflight)
internal/compose/compose.go           + BuiltServices(); shared parse()
internal/compose/compose_test.go      + BuiltServices test
internal/override/override.go         + Spec.BuiltServices; image:!reset for built
internal/override/override_test.go    + built-image test
internal/proxy/proxy.go               Up() captures stderr, translates port conflicts
cmd/up.go                             preflight; already-running no-op; pass BuiltServices
cmd/down.go                           --volumes; base-only down
cmd/restart.go                        NEW — lane restart
cmd/proxy.go, cmd/tls.go              preflight.DockerRunning gate
docs/onboarding-remind.md, README.md, CHANGELOG.md   image-isolation pattern
```

---

### Task 1: `internal/preflight` package

**Files:** Create `internal/preflight/preflight.go`, `internal/preflight/preflight_test.go`

- [ ] **Step 1: Write the failing tests**

`internal/preflight/preflight_test.go`:
```go
package preflight

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

func TestIsPortConflict(t *testing.T) {
	yes := []string{
		"Error: ... address already in use",
		`Bind for 0.0.0.0:80 failed: port is already allocated`,
	}
	for _, s := range yes {
		if !IsPortConflict(s) {
			t.Errorf("IsPortConflict(%q) = false, want true", s)
		}
	}
	if IsPortConflict("some unrelated error") {
		t.Error("IsPortConflict matched unrelated error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/preflight/`
Expected: FAIL — package/symbols undefined.

- [ ] **Step 3: Implement**

`internal/preflight/preflight.go`:
```go
// Package preflight runs environment checks shared by doctor and the action
// commands (up/proxy/tls).
package preflight

import (
	"errors"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

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

// DockerRunning returns an actionable error if the Docker daemon is unreachable.
func DockerRunning() error {
	if err := exec.Command("docker", "info").Run(); err != nil {
		return errors.New("Docker doesn't appear to be running — start Docker and retry")
	}
	return nil
}

// ComposeOK runs `docker compose version` and reports whether it is >= 2.20,
// along with the raw version line.
func ComposeOK() (bool, string) {
	out, _ := exec.Command("docker", "compose", "version").CombinedOutput()
	line := strings.TrimSpace(string(out))
	return composeOK(line), line
}

// ComposeReady returns an actionable error if Docker Compose is too old.
func ComposeReady() error {
	if ok, line := ComposeOK(); !ok {
		return errors.New("Docker Compose >= 2.20 is required (the !reset override needs it); found: " + line)
	}
	return nil
}

// IsPortConflict reports whether command output indicates a host-port clash.
func IsPortConflict(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "address already in use") ||
		strings.Contains(s, "port is already allocated")
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/preflight/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/preflight/
git commit -m "feat(preflight): Docker/Compose/port checks shared with doctor"
```

---

### Task 2: `doctor` uses `preflight`

**Files:** Modify `internal/doctor/doctor.go`; delete `internal/doctor/doctor_test.go`

- [ ] **Step 1: Replace the compose check + drop local version logic**

In `internal/doctor/doctor.go`:
- Add import `"github.com/dheeraj-nalapat/lane/internal/preflight"`.
- Delete `var verRe = ...` and the `composeOK` func.
- Remove imports `"regexp"` and `"strconv"` (now unused).
- In `Run`, replace the compose check lines:
```go
	cv, _ := cmdOut("docker", "compose", "version")
	checks = append(checks, Check{"compose >= 2.20", composeOK(cv), "upgrade Docker Compose"})
```
with:
```go
	okCompose, _ := preflight.ComposeOK()
	checks = append(checks, Check{"compose >= 2.20", okCompose, "upgrade Docker Compose"})
```

- [ ] **Step 2: Delete the moved test**

Run: `git rm internal/doctor/doctor_test.go`
(`TestComposeOK` now lives in `preflight`.)

- [ ] **Step 3: Build + test + vet**

Run: `go build ./... && go test ./internal/doctor/ ./internal/preflight/ && go vet ./internal/doctor/`
Expected: builds; tests pass; vet clean (no unused imports).

- [ ] **Step 4: Commit**

```bash
git add internal/doctor/
git commit -m "refactor(doctor): source compose check from preflight"
```

---

### Task 3: `compose.BuiltServices`

**Files:** Modify `internal/compose/compose.go`, `internal/compose/compose_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/compose/compose_test.go`:
```go
func TestBuiltServices(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "docker-compose.yml")
	body := `services:
  server:
    image: app/server
    build:
      context: .
  redis:
    image: redis:7
`
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := BuiltServices(p)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(got) != 1 || got[0] != "server" {
		t.Fatalf("BuiltServices = %v, want [server]", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/compose/ -run TestBuiltServices`
Expected: FAIL — `undefined: BuiltServices`.

- [ ] **Step 3: Implement (struct gains build; shared parse)**

Replace `internal/compose/compose.go` with:
```go
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
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/compose/`
Expected: PASS (existing `TestServiceNames` still green).

- [ ] **Step 5: Commit**

```bash
git add internal/compose/
git commit -m "feat(compose): BuiltServices detects services with a build section"
```

---

### Task 4: Override resets `image:` for built services

**Files:** Modify `internal/override/override.go`, `internal/override/override_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/override/override_test.go`:
```go
func TestGenerate_ResetsBuiltImage(t *testing.T) {
	out, _ := Generate(Spec{
		Slug: "demo", ProjectPath: "/p", Network: "lane",
		Services: []string{"server", "redis"}, BuiltServices: []string{"server"},
		Routes: []Route{{Service: "server", Port: 8000, Hostname: "demo.localhost"}},
	})
	s := string(out)
	if !strings.Contains(s, "image: !reset null") {
		t.Fatalf("built service should reset image:\n%s", s)
	}
	// exactly one image reset (server, not redis)
	if c := strings.Count(s, "image: !reset null"); c != 1 {
		t.Fatalf("got %d image resets, want 1", c)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/override/ -run TestGenerate_ResetsBuiltImage`
Expected: FAIL — `Spec.BuiltServices undefined`.

- [ ] **Step 3: Implement**

In `internal/override/override.go`, add to `Spec` (after `TLS bool`):
```go
	BuiltServices []string
```
At the top of `Generate` (after the `routed` map is built), add a set:
```go
	built := map[string]bool{}
	for _, b := range s.BuiltServices {
		built[b] = true
	}
```
Inside the per-service loop, right after the `svc := map[string]any{...}` literal, add:
```go
		if built[name] {
			svc["image"] = resetNode{} // !reset null → compose names the built image by project (slug)
		}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/override/`
Expected: PASS (TLS/other tests unaffected; `BuiltServices` defaults empty → no reset).

- [ ] **Step 5: Commit**

```bash
git add internal/override/
git commit -m "feat(override): reset image for built services (per-slug isolation)"
```

---

### Task 5: Proxy translates port conflicts

**Files:** Modify `internal/proxy/proxy.go`

- [ ] **Step 1: Capture stderr + translate in `Up`**

In `internal/proxy/proxy.go`, add import `"github.com/dheeraj-nalapat/lane/internal/preflight"`.
Replace the final `return dockerCompose("up", "-d")` in `Up()` with a capturing run:
```go
	out, err := exec.Command("docker", "compose", "-p", "lane-proxy", "-f", composePath(), "up", "-d").CombinedOutput()
	if err != nil {
		if preflight.IsPortConflict(string(out)) {
			return fmt.Errorf("a required port (80/443) is in use by another process — free it or stop that service, then `lane proxy up`\n%s", out)
		}
		return fmt.Errorf("starting lane proxy: %v\n%s", err, out)
	}
	return nil
```
(`dockerCompose` stays for `Down`.)

- [ ] **Step 2: Build + vet + existing tests**

Run: `go build ./... && go vet ./internal/proxy/ && go test ./internal/proxy/`
Expected: builds; vet clean; renderCompose tests still pass.

- [ ] **Step 3: Commit**

```bash
git add internal/proxy/
git commit -m "feat(proxy): translate host-port conflicts into an actionable error"
```

---

### Task 6: `cmd/up.go` — preflight, no-op-when-running, built services

**Files:** Modify `cmd/up.go`

- [ ] **Step 1: Add imports**

Add to `cmd/up.go` imports: `"github.com/dheeraj-nalapat/lane/internal/preflight"`.

- [ ] **Step 2: Preflight at the top of `runUp`**

Immediately after `dir, err := projectDir(args)` block, add:
```go
	if err := preflight.DockerRunning(); err != nil {
		return err
	}
	if err := preflight.ComposeReady(); err != nil {
		return err
	}
```

- [ ] **Step 3: No-op when the same-path stack already runs**

Replace the existing collision block:
```go
	if claimed, ok := dockerx.SlugOwner(sl); ok && claimed != dir {
		return fmt.Errorf("slug %q already in use by stack at %s; pass --slug to disambiguate", sl, claimed)
	}
```
with:
```go
	if claimed, ok := dockerx.SlugOwner(sl); ok {
		if claimed == dir {
			fmt.Printf("stack %q already running — use `lane restart` to recreate, or `lane down` to stop\n", sl)
			return nil
		}
		return fmt.Errorf("slug %q already in use by stack at %s; pass --slug to disambiguate", sl, claimed)
	}
```

- [ ] **Step 4: Pass BuiltServices to the override**

After `svcs, err := compose.ServiceNames(composePath)` (and its error check), add:
```go
	built, err := compose.BuiltServices(composePath)
	if err != nil {
		return err
	}
```
Add `BuiltServices: built,` to the `override.Spec{...}` literal:
```go
	body, err := override.Generate(override.Spec{
		Slug: sl, ProjectPath: dir, Network: proxy.Network,
		Services: svcs, Routes: routes, TiltPort: tiltPort, TLS: tlsOn,
		BuiltServices: built,
	})
```

- [ ] **Step 5: Build + full test + vet**

Run: `go build ./... && go test ./... && go vet ./...`
Expected: builds, all pass, vet clean.

- [ ] **Step 6: Commit**

```bash
git add cmd/up.go
git commit -m "feat(up): preflight gate, no-op when already running, isolate built images"
```

---

### Task 7: `cmd/down.go` — `--volumes`, base-only teardown

**Files:** Modify `cmd/down.go`

- [ ] **Step 1: Add the flag**

In `cmd/down.go`, add a package var and register it:
```go
var flagDownVolumes bool

func init() {
	downCmd.Flags().BoolVar(&flagDownVolumes, "volumes", false, "also remove the stack's named volumes (data reset)")
	root.AddCommand(downCmd)
}
```
(Replace the existing `func init() { root.AddCommand(downCmd) }`.)

- [ ] **Step 2: Base-only down + optional --volumes**

Replace the teardown block (the `overridePath`/`composePath`/`dc` section) with:
```go
	composePath := filepath.Join(dir, m.ComposeFile)
	args := []string{"compose", "-p", sl, "-f", composePath, "down", "--remove-orphans"}
	if flagDownVolumes {
		args = append(args, "--volumes")
	}
	dc := exec.Command("docker", args...)
	dc.Stdout, dc.Stderr = os.Stdout, os.Stderr
	if err := dc.Run(); err != nil {
		return err
	}

	_ = os.Remove(filepath.Join(paths.Overrides(), sl+".override.yml"))
	_ = os.Remove(filepath.Join(paths.TraefikDynamic(), sl+".yml"))
	msg := sl + " torn down"
	if flagDownVolumes {
		msg += " (with volumes)"
	}
	fmt.Printf("lane: %s\n", msg)
	return nil
```
(Down no longer requires the override file — it removes the project by name from the base compose, so `restart` works even when no override exists.)

- [ ] **Step 3: Build + vet**

Run: `go build ./... && go vet ./...`
Expected: builds, vet clean.

- [ ] **Step 4: Commit**

```bash
git add cmd/down.go
git commit -m "feat(down): --volumes flag; base-only teardown (no override needed)"
```

---

### Task 8: `lane restart`

**Files:** Create `cmd/restart.go`

- [ ] **Step 1: Implement**

`cmd/restart.go`:
```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart [path]",
	Short: "Recreate a stack (down then up)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Best-effort teardown (a not-running stack is fine), then bring up fresh.
		if err := runDown(cmd, args); err != nil {
			fmt.Fprintln(os.Stderr, "lane: restart: down step:", err)
		}
		return runUp(cmd, args)
	},
}

func init() { root.AddCommand(restartCmd) }
```

- [ ] **Step 2: Build + vet**

Run: `go build ./... && go vet ./...`
Expected: builds, vet clean.

- [ ] **Step 3: Commit**

```bash
git add cmd/restart.go
git commit -m "feat: lane restart (down then up)"
```

---

### Task 9: Preflight gate on `proxy up` + `tls enable`

**Files:** Modify `cmd/proxy.go`, `cmd/tls.go`

- [ ] **Step 1: `proxy up`**

In `cmd/proxy.go`, add import `"github.com/dheeraj-nalapat/lane/internal/preflight"`. In the `case "up":` branch, before `proxy.Up()`:
```go
		case "up":
			if err := preflight.DockerRunning(); err != nil {
				return err
			}
			if err := proxy.Up(); err != nil {
				return err
			}
```

- [ ] **Step 2: `tls enable`**

In `cmd/tls.go`, add import `"github.com/dheeraj-nalapat/lane/internal/preflight"`. At the start of `tlsEnable()`, before the mkcert checks:
```go
	if err := preflight.DockerRunning(); err != nil {
		return err
	}
```

- [ ] **Step 3: Build + vet + test**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: builds, vet clean, all pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/proxy.go cmd/tls.go
git commit -m "feat(proxy,tls): gate on Docker-running preflight"
```

---

### Task 10: Docs — image isolation pattern + CHANGELOG

**Files:** Modify `docs/onboarding-remind.md`, `README.md`, `CHANGELOG.md`, `/home/dheerajnalapat/project/ReMind/Tiltfile`

- [ ] **Step 1: Enable the tag hook in ReMind's Tiltfile (uncommitted, for review)**

In `/home/dheerajnalapat/project/ReMind/Tiltfile`, change:
```python
    tag = ""  # v1: per-slug image tags disabled
```
to:
```python
    tag = (":" + lane_slug) if lane_slug else ""  # per-slug image isolation
```

- [ ] **Step 2: Document the pattern**

In `docs/onboarding-remind.md`, under the Tiltfile contract section, add:
```markdown
### Per-slug image isolation (tilt projects)

Tilt builds images by the ref in your Tiltfile. To keep worktrees from sharing
(and clobbering) a built image tag, suffix built refs with the slug:

    tag = (":" + lane_slug) if lane_slug else ""
    docker_build("remind/platform-server" + tag, ...)

(Compose-runner projects get this automatically — lane resets the `image:` of any
`build:` service so Compose names it per project/slug.)
```

In `README.md`, add a short bullet under the relevant section noting that the
compose runner isolates built images automatically and tilt projects use the
`tag` pattern above.

- [ ] **Step 3: CHANGELOG**

In `CHANGELOG.md` under `## [Unreleased]` → `### Added`, append:
```markdown
- Robustness: actionable preflight errors (Docker not running, Compose < 2.20,
  host-port conflicts); `lane up` no-ops when the stack is already running;
  `lane restart`; `lane down --volumes`; per-slug built-image isolation
  (automatic for the compose runner).
```

- [ ] **Step 4: Commit (lane repo only; ReMind stays uncommitted for the owner)**

```bash
cd /home/dheerajnalapat/project/lane
git add docs/onboarding-remind.md README.md CHANGELOG.md
git commit -m "docs: per-slug image isolation + C1 changelog"
```

---

### Task 11: Live / integration smoke (needs Docker)

**No code. Verification.**

- [ ] **Step 1: Build + preflight messages**

```bash
go build -o ~/.local/bin/lane . && hash -r
# (optional) stop Docker → `lane up` in a project prints the daemon message, non-zero exit; restart Docker after.
```

- [ ] **Step 2: Idempotent up + restart (use a whoami compose project)**

```bash
lane proxy up
cd <project> && lane up           # compose runner
lane up                           # → "stack <slug> already running ..."; no duplicate
docker ps --filter label=lane.slug=<slug> --format '{{.Names}}' | sort   # same container set
lane restart                      # recreates
```

- [ ] **Step 3: down --volumes**

```bash
docker volume ls | grep <slug> || echo "(no named volumes for this project)"
lane down --volumes               # message says "(with volumes)"
docker volume ls | grep <slug> && echo "STILL PRESENT (unexpected)" || echo "volumes gone"
```

- [ ] **Step 4: Built-image isolation (compose runner)**

Use a project with a `build:`+`image:` service across two slugs:
```bash
(cd <proj> && lane up); (cd <proj> && lane up --slug alt)
docker images | grep <project-or-slug>   # two distinct slug-named built images
(cd <proj> && lane down); (cd <proj> && lane down --slug alt)
```

- [ ] **Step 5: Tear down**

```bash
cd <project> && lane down ; lane proxy down
```

---

## Final verification

- [ ] `go test ./...` — all pass (preflight, compose, override, plus existing); `go vet ./...` clean; `gofmt -l .` empty.
- [ ] `lane up` twice → second no-ops, no duplicate stack.
- [ ] `lane restart` recreates; `lane down --volumes` removes volumes.
- [ ] Compose `build:` service gets a per-slug image; pulled images untouched.
- [ ] Docker stopped → `lane up`/`proxy up` print the daemon message (non-zero exit).
- [ ] Happy path unchanged: a normal `up` with healthy env behaves exactly as before.
