# Agent Integration (G) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make lane fully agent-drivable for parallel testing — `--json` output, `--wait` readiness, concurrency-safe parallel `up` — and ship a Claude Code skill (packaged as a plugin + marketplace) and a Cursor rule.

**Architecture:** Two pure, testable internal packages (`ready` for HTTP-poll readiness, `lockfile` for a portable advisory lock); `proxy.Up` wraps shared bring-up in the lock; `up`/`ls` gain `--json` (stdout JSON, human → stderr) and `up` gains `--wait`/`--wait-timeout` (implies detach). Skill files live under `agent/` with a repo-as-marketplace manifest.

**Tech Stack:** Go 1.22 stdlib (`net/http`, `os`, `encoding/json`); no new deps. Claude Code plugin/skill + Cursor `.mdc` formats.

Spec: `docs/2026-06-09-agent-integration-design.md`.

---

## File Structure

```
internal/ready/ready.go            NEW — WaitReady(urls, timeout, client)
internal/ready/ready_test.go       NEW
internal/lockfile/lockfile.go      NEW — Acquire(path, timeout) → release
internal/lockfile/lockfile_test.go NEW
internal/proxy/proxy.go            Up() wraps ensureNetwork+compose in the lock
internal/stack/stack.go            + json tags
cmd/ls.go                          --json
internal/runner/runner.go          + RunSpec.Quiet; emit() helper (stderr when quiet)
internal/runner/compose.go,tilt.go use emit() instead of fmt.Printf
cmd/up.go                          --json/--wait/--wait-timeout; UpResult; restructure
agent/claude/.claude-plugin/plugin.json, agent/claude/skills/lane/SKILL.md  NEW
agent/cursor/lane.mdc              NEW
.claude-plugin/marketplace.json    NEW
README.md, CHANGELOG.md            agent section + entry
```

---

### Task 1: `internal/ready` — readiness poller

**Files:** Create `internal/ready/ready.go`, `internal/ready/ready_test.go`

- [ ] **Step 1: Write the failing test**

`internal/ready/ready_test.go`:
```go
package ready

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitReady_BecomesReady(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&n, 1) < 2 {
			w.WriteHeader(http.StatusBadGateway) // 502 first
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	if err := WaitReady([]string{srv.URL}, 3*time.Second, srv.Client()); err != nil {
		t.Fatalf("WaitReady: %v", err)
	}
}

func TestWaitReady_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	if err := WaitReady([]string{srv.URL}, 600*time.Millisecond, srv.Client()); err == nil {
		t.Fatal("expected timeout error")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/ready/`
Expected: FAIL — `undefined: WaitReady`.

- [ ] **Step 3: Implement**

`internal/ready/ready.go`:
```go
// Package ready polls routed URLs until a stack is serving HTTP.
package ready

import (
	"fmt"
	"net/http"
	"time"
)

// WaitReady blocks until every URL returns an HTTP response with status < 500
// (i.e. the route exists and the backend is up — not a Traefik 502/503), or the
// timeout elapses. A nil client gets a default 3s-per-request client.
func WaitReady(urls []string, timeout time.Duration, client *http.Client) error {
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	deadline := time.Now().Add(timeout)
	for _, u := range urls {
		for {
			if probe(client, u) {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("timed out waiting for %s to become ready", u)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}

func probe(client *http.Client, u string) bool {
	resp, err := client.Get(u)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode < 500
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/ready/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ready/
git commit -m "feat(ready): HTTP-poll readiness for routed URLs"
```

---

### Task 2: `internal/lockfile` — portable advisory lock

**Files:** Create `internal/lockfile/lockfile.go`, `internal/lockfile/lockfile_test.go`

- [ ] **Step 1: Write the failing test**

`internal/lockfile/lockfile_test.go`:
```go
package lockfile

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAcquire_MutualExclusion(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.lock")
	rel, err := Acquire(p, time.Second)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if _, err := Acquire(p, 300*time.Millisecond); err == nil {
		t.Fatal("second acquire should fail while held")
	}
	rel()
	rel2, err := Acquire(p, time.Second)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	rel2()
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/lockfile/`
Expected: FAIL — `undefined: Acquire`.

- [ ] **Step 3: Implement**

`internal/lockfile/lockfile.go`:
```go
// Package lockfile is a portable advisory lock via O_CREATE|O_EXCL, with a
// stale-lock reclaim. Used to serialize shared-infra bring-up across parallel
// `lane up` invocations.
package lockfile

import (
	"fmt"
	"os"
	"time"
)

const staleAfter = 30 * time.Second

// Acquire blocks until it can create path exclusively or timeout elapses.
// Returns a release func that removes the lock.
func Acquire(path string, timeout time.Duration) (func(), error) {
	deadline := time.Now().Add(timeout)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			_ = f.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if fi, statErr := os.Stat(path); statErr == nil && time.Since(fi.ModTime()) > staleAfter {
			_ = os.Remove(path) // reclaim a stale lock (owner likely died)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("could not acquire lock %s (held by another lane process)", path)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
```

- [ ] **Step 4: Run test**

Run: `go test ./internal/lockfile/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/lockfile/
git commit -m "feat(lockfile): portable advisory lock with stale reclaim"
```

---

### Task 3: Lock the shared proxy bring-up

**Files:** Modify `internal/proxy/proxy.go`

- [ ] **Step 1: Wrap `Up` in the lock**

In `internal/proxy/proxy.go`, add imports `"path/filepath"`, `"time"`, and
`"github.com/dheeraj-nalapat/lane/internal/lockfile"`. Replace the start of `Up`:
```go
func Up() error {
	if err := paths.Ensure(); err != nil {
		return err
	}
```
with:
```go
func Up() error {
	if err := paths.Ensure(); err != nil {
		return err
	}
	release, err := lockfile.Acquire(filepath.Join(paths.Home(), "proxy.lock"), 30*time.Second)
	if err != nil {
		return err
	}
	defer release()
```
(Leave the rest of `Up` unchanged — `ensureNetwork`, render, write, compose up now run under the lock.)

- [ ] **Step 2: Build + vet + existing tests**

Run: `go build ./... && go vet ./internal/proxy/ && go test ./internal/proxy/`
Expected: builds; vet clean; renderCompose tests still pass.

- [ ] **Step 3: Commit**

```bash
git add internal/proxy/
git commit -m "feat(proxy): serialize shared bring-up with a lock (parallel-safe)"
```

---

### Task 4: `stack` JSON tags + `lane ls --json`

**Files:** Modify `internal/stack/stack.go`, `cmd/ls.go`

- [ ] **Step 1: Add JSON tags to Stack**

Replace `internal/stack/stack.go` body's struct with:
```go
// Stack is one lane-managed project stack, aggregated from container labels.
type Stack struct {
	Slug        string   `json:"slug"`
	URL         string   `json:"url"`
	TiltPort    int      `json:"tiltPort"`
	ProjectPath string   `json:"path"`
	Containers  []string `json:"-"`
	Running     bool     `json:"running"`
}
```

- [ ] **Step 2: Add `--json` to `ls`**

Replace `cmd/ls.go` with:
```go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/dheeraj-nalapat/lane/internal/dockerx"
	"github.com/dheeraj-nalapat/lane/internal/stack"
	"github.com/spf13/cobra"
)

var flagLsJSON bool

var lsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List running lane stacks",
	RunE: func(cmd *cobra.Command, args []string) error {
		stacks, err := dockerx.List()
		if err != nil {
			return err
		}
		if flagLsJSON {
			if stacks == nil {
				stacks = []stack.Stack{} // marshal [] not null
			}
			b, err := json.MarshalIndent(stacks, "", "  ")
			if err != nil {
				return err
			}
			fmt.Println(string(b))
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "SLUG\tURL\tTILT\tSTATE\tPATH")
		for _, s := range stacks {
			state := "stopped"
			if s.Running {
				state = "running"
			}
			tilt := "-"
			if s.TiltPort > 0 {
				tilt = fmt.Sprintf("%d", s.TiltPort)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.Slug, s.URL, tilt, state, s.ProjectPath)
		}
		return w.Flush()
	},
}

func init() {
	lsCmd.Flags().BoolVar(&flagLsJSON, "json", false, "output as JSON")
	root.AddCommand(lsCmd)
}
```

- [ ] **Step 3: Build + manual**

```bash
go build ./... && go vet ./...
go build -o ./bin/lane . && ./bin/lane ls --json   # → [] (or current stacks) as JSON
```
Expected: valid JSON array printed.

- [ ] **Step 4: Commit**

```bash
git add internal/stack/ cmd/ls.go
git commit -m "feat(ls): --json output; json tags on Stack"
```

---

### Task 5: Runner quiet mode (human text → stderr under `--json`)

**Files:** Modify `internal/runner/runner.go`, `internal/runner/compose.go`, `internal/runner/tilt.go`

- [ ] **Step 1: Add `Quiet` + an `emit` helper**

In `internal/runner/runner.go`: add `Quiet bool` to `RunSpec` (after `TLS bool`),
add `"os"` to imports, and add a helper + rewrite `printURLs` to use it:
```go
// emit writes human/progress text to stdout, or stderr when Quiet (so --json
// stdout stays machine-clean).
func emit(s RunSpec, format string, a ...any) {
	w := os.Stdout
	if s.Quiet {
		w = os.Stderr
	}
	fmt.Fprintf(w, format, a...)
}

func printURLs(s RunSpec) {
	emit(s, "lane: %s\n", s.Slug)
	for _, r := range s.Routes {
		emit(s, "  → http://%s  (%s:%d)\n", r.Hostname, r.Service, r.Port)
	}
}
```

- [ ] **Step 2: Route the runners' messages through `emit`**

In `internal/runner/compose.go`, replace `fmt.Printf("up (detached)...` with:
```go
	emit(s, "up (detached). logs: lane logs --slug %s\n", s.Slug)
```
(Remove the now-unused `fmt` import only if nothing else uses it — `fmt` is still
used elsewhere; keep it.)

In `internal/runner/tilt.go`, replace the two `fmt.Printf` calls:
```go
	emit(s, "  → http://tilt-%s.localhost  (Tilt UI)\n", s.Slug)
```
and
```go
		emit(s, "detached (pid %d). logs: lane logs --slug %s\n", tcmd.Process.Pid, s.Slug)
```

- [ ] **Step 3: Build + tests**

Run: `go build ./... && go test ./internal/runner/`
Expected: builds; tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/runner/
git commit -m "feat(runner): Quiet mode routes human text to stderr"
```

---

### Task 6: `lane up --json / --wait`

**Files:** Modify `cmd/up.go`

- [ ] **Step 1: Add flags + UpResult + helper**

In `cmd/up.go`, add imports `"encoding/json"` and `"time"`, plus
`"github.com/dheeraj-nalapat/lane/internal/ready"`. Add flags + types near the top:
```go
var (
	flagJSON        bool
	flagWait        bool
	flagWaitTimeout time.Duration
)

type upURL struct {
	Service string `json:"service"`
	Host    string `json:"host"`
	URL     string `json:"url"`
}
type upResult struct {
	Slug    string  `json:"slug"`
	Runner  string  `json:"runner"`
	TLS     bool    `json:"tls"`
	TiltURL string  `json:"tiltUrl,omitempty"`
	URLs    []upURL `json:"urls"`
}

func buildUpResult(slug, runnerName string, tlsOn bool, routes []override.Route, tiltPort int) upResult {
	res := upResult{Slug: slug, Runner: runnerName, TLS: tlsOn}
	for _, r := range routes {
		res.URLs = append(res.URLs, upURL{Service: r.Service, Host: r.Hostname, URL: "http://" + r.Hostname})
	}
	if tiltPort > 0 {
		res.TiltURL = "http://tilt-" + slug + ".localhost"
	}
	return res
}

func routeURLs(routes []override.Route) []string {
	var u []string
	for _, r := range routes {
		u = append(u, "http://"+r.Hostname)
	}
	return u
}
```
Register the flags in `init()`:
```go
	upCmd.Flags().BoolVar(&flagJSON, "json", false, "print the result as JSON (implies detach)")
	upCmd.Flags().BoolVar(&flagWait, "wait", false, "wait until routes are serving before returning (implies detach)")
	upCmd.Flags().DurationVar(&flagWaitTimeout, "wait-timeout", 90*time.Second, "max time to wait with --wait")
```

- [ ] **Step 2: Build routes earlier + handle the no-op/JSON paths**

Move the `routes` construction to **before** the already-running check. Replace
the block from `sl := slug.Resolve(...)` through the collision check with:
```go
	sl := slug.Resolve(slug.Inputs{
		Flag: flagSlug, Env: os.Getenv("LANE_SLUG"),
		ManifestName: m.Name, Worktree: wt, DirBase: filepath.Base(dir),
	})

	var routes []override.Route
	for _, r := range m.Routes {
		routes = append(routes, override.Route{
			Service: r.Service, Port: r.Port,
			Hostname: identity.RenderHost(r.Host, sl),
		})
	}
	runnerName := runner.Select(m.Runner, tiltfileExists(dir))

	if claimed, ok := dockerx.SlugOwner(sl); ok {
		if claimed == dir {
			if flagJSON {
				return printJSON(buildUpResult(sl, runnerName, tlsx.Enabled(), routes, 0))
			}
			fmt.Printf("stack %q already running — use `lane restart` to recreate, or `lane down` to stop\n", sl)
			return nil
		}
		return fmt.Errorf("slug %q already in use by stack at %s; pass --slug to disambiguate", sl, claimed)
	}
```
Delete the later duplicate `runnerName := runner.Select(...)` line and the later
`routes` loop (now built above). Keep the `m.Runner == "tilt" && !tiltfileExists`
warning where the old `runnerName` line was.

Add a tiny helper:
```go
func printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
```

- [ ] **Step 3: Detach implication + Quiet + spec**

Where the `spec` is built, compute the agent-mode flags and set them:
```go
	wantResult := flagJSON || flagWait
	detach := flagDetach || (wantResult && runnerName == "tilt")

	spec := runner.RunSpec{
		Slug: sl, Dir: dir, ComposePath: composePath, OverridePath: overridePath,
		Routes: routes, Detach: detach, Build: flagBuild,
		TiltPort: tiltPort, DynamicPath: dynamicPath, Env: env, TLS: tlsOn,
		Quiet: flagJSON,
	}
```
(Replace the existing `Detach: flagDetach` with `Detach: detach` and add `Quiet`.)

- [ ] **Step 4: Run, then wait + JSON after `r.Up`**

Replace the final `return r.Up(spec)` with:
```go
	if err := r.Up(spec); err != nil {
		return err
	}
	if flagWait {
		if err := ready.WaitReady(routeURLs(routes), flagWaitTimeout, nil); err != nil {
			return err
		}
	}
	if flagJSON {
		return printJSON(buildUpResult(sl, runnerName, tlsOn, routes, tiltPort))
	}
	return nil
```

- [ ] **Step 5: Build + full test + vet**

Run: `go build ./... && go test ./... && go vet ./...`
Expected: builds, all pass, vet clean.

- [ ] **Step 6: Dry-run JSON shape sanity (no Docker needed for --dry-run? it hits dockerx.SlugOwner)**

```bash
go build -o ./bin/lane .
# In a project, with the stack NOT running, dry-run still prints the override (not JSON);
# JSON is emitted on real up. Just verify build + help:
./bin/lane up --help | grep -E 'json|wait'
```
Expected: `--json`, `--wait`, `--wait-timeout` listed.

- [ ] **Step 7: Commit**

```bash
git add cmd/up.go
git commit -m "feat(up): --json result, --wait readiness (imply detach); machine-clean stdout"
```

---

### Task 7: Skill / plugin / Cursor / marketplace files

**Files:** Create `agent/claude/.claude-plugin/plugin.json`, `agent/claude/skills/lane/SKILL.md`, `agent/cursor/lane.mdc`, `.claude-plugin/marketplace.json`

- [ ] **Step 1: Plugin manifest**

`agent/claude/.claude-plugin/plugin.json`:
```json
{
  "name": "lane",
  "description": "Run isolated, port-conflict-free dev stacks per git worktree for parallel testing.",
  "version": "0.1.0",
  "author": { "name": "Dheeraj Nalapat" }
}
```

- [ ] **Step 2: Claude skill**

`agent/claude/skills/lane/SKILL.md`:
```markdown
---
name: lane
description: Use when running or testing multiple services, or the same project across several git worktrees, in parallel — lane spins up isolated, port-conflict-free stacks each reachable at a friendly *.localhost URL.
---

# Testing in parallel with lane

`lane` brings a project's Docker/Tilt stack up behind a shared proxy, so **each
git worktree gets its own isolated stack with no host-port conflicts**. Multiple
agents (one per worktree) can bring stacks up and test concurrently.

## The loop (per worktree)

1. Bring the stack up and wait until it's serving, getting machine-readable URLs:
   ```bash
   lane up --wait --json
   ```
   Parse stdout JSON: `{"slug":"...","urls":[{"url":"http://<slug>.localhost"}], ...}`.
   (`--json`/`--wait` imply detached; human logs go to stderr; exit code 0 = ready.)
2. Run tests / requests against the returned `url`(s).
3. Tear down when done:
   ```bash
   lane down
   ```

## Notes

- `lane ls --json` lists running stacks. Exit code `0` = success (including
  "already running"); `1` = error.
- The slug derives from the git worktree, so two worktrees of one repo get
  distinct URLs automatically — no port coordination needed.
- First time only: `lane doctor` checks the environment; `lane proxy up` starts
  the shared proxy (lane does this automatically on `up`).
```

- [ ] **Step 3: Cursor rule**

`agent/cursor/lane.mdc`:
```markdown
---
description: Use lane to run/test stacks in parallel across git worktrees (isolated, no port conflicts).
alwaysApply: false
---

When you need to run or test this project (especially across multiple git
worktrees in parallel), use `lane`:

- `lane up --wait --json` — bring up an isolated stack for the current worktree,
  wait until it serves, and print `{slug, urls[]}` JSON (human logs on stderr).
- Test against the returned `url` (e.g. `http://<slug>.localhost`).
- `lane down` — tear the stack down.
- `lane ls --json` — list running stacks. Exit 0 = ok, 1 = error.

Each worktree gets its own slug/URL, so parallel agents never collide on ports.
```

- [ ] **Step 4: Marketplace manifest (repo-as-marketplace)**

`.claude-plugin/marketplace.json`:
```json
{
  "$schema": "https://anthropic.com/claude-code/marketplace.schema.json",
  "name": "lane",
  "description": "lane — parallel, port-conflict-free dev stacks for agents.",
  "owner": { "name": "Dheeraj Nalapat" },
  "plugins": [
    {
      "name": "lane",
      "description": "Run isolated dev stacks per git worktree for parallel testing.",
      "category": "development",
      "source": {
        "source": "git-subdir",
        "url": "https://github.com/Dheeraj-Nalapat/lane.git",
        "path": "agent/claude",
        "ref": "main"
      }
    }
  ]
}
```

- [ ] **Step 5: Sanity-check the JSON parses**

Run:
```bash
for f in agent/claude/.claude-plugin/plugin.json .claude-plugin/marketplace.json; do
  python3 -c "import json,sys; json.load(open('$f'))" && echo "$f ok"
done
head -3 agent/claude/skills/lane/SKILL.md   # frontmatter present
```
Expected: both `ok`; SKILL.md starts with `---`.

- [ ] **Step 6: Commit**

```bash
git add agent/ .claude-plugin/
git commit -m "feat(agent): Claude skill (plugin + marketplace) and Cursor rule for parallel testing"
```

---

### Task 8: Docs

**Files:** Modify `README.md`, `CHANGELOG.md`

- [ ] **Step 1: README — add the agent section**

After the "HTTPS (optional)" section in `README.md`, add:
```markdown
## Using lane with coding agents (parallel testing)

lane gives each git worktree an isolated, port-conflict-free stack, so multiple
agents (Claude, Cursor, …) can test in parallel. The agent loop:

```bash
lane up --wait --json   # isolated stack for this worktree; waits until serving; prints {slug, urls[]}
# ...run tests against the returned url...
lane down
```

`--json` prints machine-readable output on stdout (human logs on stderr); exit
`0` = success (incl. already-running), `1` = error. `lane ls --json` lists
stacks. N parallel `lane up`s are race-safe (the shared proxy bring-up is
locked).

**Skill files:** a Claude Code skill (`agent/claude`, installable as a plugin —
`/plugin marketplace add Dheeraj-Nalapat/lane`) and a Cursor rule
(`agent/cursor/lane.mdc`).
```

- [ ] **Step 2: CHANGELOG**

Under `## [Unreleased]` → `### Added`, append:
```markdown
- Agent integration: `lane up --json`/`--wait` and `lane ls --json` for
  machine-driven use; race-safe parallel `up` (locked proxy bring-up); a Claude
  Code skill (packaged as a plugin + marketplace) and a Cursor rule documenting
  the worktree → `lane up --wait --json` → test → `lane down` parallel loop.
```

- [ ] **Step 3: Commit**

```bash
git add README.md CHANGELOG.md
git commit -m "docs: agent integration (parallel testing) section"
```

---

### Task 9: Live smoke (needs Docker)

**No code. Verification.**

- [ ] **Step 1: Build + JSON/ls**

```bash
go build -o ~/.local/bin/lane . && hash -r
lane proxy up
lane ls --json | python3 -m json.tool >/dev/null && echo "ls --json valid"
```

- [ ] **Step 2: up --wait --json (whoami project)**

```bash
mkdir -p /tmp/lane-agent && cd /tmp/lane-agent
printf 'services:\n  web:\n    image: traefik/whoami\n' > docker-compose.yml
printf 'name="agentdemo"\ncompose_file="docker-compose.yml"\n[[routes]]\nservice="web"\nport=80\n' > .lane.toml
OUT=$(lane up --wait --json) ; echo "$OUT"
echo "$OUT" | python3 -c "import json,sys; d=json.load(sys.stdin); print('url=', d['urls'][0]['url'])"
# The --wait guarantees it's serving: this curl should already be 200
curl -s -o /dev/null -w 'ready check: %{http_code}\n' "$(echo "$OUT" | python3 -c 'import json,sys;print(json.load(sys.stdin)["urls"][0]["url"])')"
```
Expected: `OUT` is clean JSON (no human text on stdout); URL prints; curl → 200 immediately (no sleep needed).

- [ ] **Step 3: Parallel race-safety (two slugs at once)**

```bash
( cd /tmp/lane-agent && lane up --slug par-a --wait --json >/tmp/a.json 2>/dev/null ) &
( cd /tmp/lane-agent && lane up --slug par-b --wait --json >/tmp/b.json 2>/dev/null ) &
wait
python3 -c "import json; print('a', json.load(open('/tmp/a.json'))['slug']); print('b', json.load(open('/tmp/b.json'))['slug'])"
lane ls --json | python3 -c "import json,sys; print('stacks:', [s['slug'] for s in json.load(sys.stdin)])"
```
Expected: both JSON files valid; both stacks present; no proxy/network error from the race.

- [ ] **Step 4: Tear down**

```bash
cd /tmp/lane-agent
lane down --slug agentdemo; lane down --slug par-a; lane down --slug par-b
cd / && rm -rf /tmp/lane-agent
```

---

## Final verification

- [ ] `go test ./...` all pass (ready, lockfile, plus existing); `go vet ./...` clean; `gofmt -l .` empty.
- [ ] `lane up --json` emits only JSON on stdout (human text on stderr); `lane ls --json` valid.
- [ ] `lane up --wait` returns only once the URL serves (curl 200 immediately after).
- [ ] Two parallel `lane up`s succeed without a proxy/network race error.
- [ ] `plugin.json` / `marketplace.json` parse; `SKILL.md` has frontmatter; Cursor rule present.
- [ ] No-flag behavior unchanged (additive).
