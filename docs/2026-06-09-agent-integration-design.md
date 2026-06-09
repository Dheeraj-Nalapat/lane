# lane — Agent Integration (G) — Design Spec

**Date:** 2026-06-09
**Status:** Design complete — ready for implementation planning.
**Sub-project:** G of the generic-release effort.

## Context

lane already gives each git worktree an isolated, port-conflict-free stack — so
**N coding agents in N worktrees can each bring up a stack and test in parallel.**
What's missing is the layer that lets agents *drive* lane reliably (machine-
readable output, readiness gating, race-safe parallel startup) and *know the
recipe* (skill/instruction files). That's the stated main intent: increase
testing speed via parallel agents.

## Goal

Make lane fully agent-drivable for parallel testing, and ship skill files
(Claude Code + Cursor) — packaged so the Claude skill can be published to a
skills marketplace.

## Decisions

| Item | Decision |
|---|---|
| `--json` | On `up` and `ls`; JSON to stdout, human logs to stderr |
| `--wait` | `lane up --wait` blocks until routes serve HTTP (timeout); **implies detach** |
| Readiness | **HTTP-poll the routed URLs through the proxy** (not Docker healthchecks) |
| Concurrency | File lock around shared proxy/network bring-up (portable, dependency-free) |
| Exit codes | `0` success (incl. already-running); `1` error; documented |
| Skill artifacts | Claude Code skill + Cursor rule (no AGENTS.md / generator in v1) |
| Marketplace | Package the Claude skill as a plugin + marketplace manifest in the repo |

## Non-goals (v1)

- `AGENTS.md` and a `lane agent init` generator (easy later from the same
  content); MCP server; per-agent auth.

## 1. `--json` + quiet + exit codes

- **`lane up --json`** → after bringing the stack up, print one JSON object to
  **stdout**; all human/progress text goes to **stderr**:
  ```json
  {"slug":"remind","runner":"compose","tls":false,
   "tiltUrl":"http://tilt-remind.localhost",
   "urls":[{"service":"ui","host":"remind.localhost","url":"http://remind.localhost"}]}
  ```
  (`tiltUrl` omitted/empty for the compose runner.)
- **`lane ls --json`** → a JSON array: `[{"slug","url","tiltPort","running","path"}]`.
- `--json` implies quiet stdout (only the JSON). Internally, an `UpResult` struct
  (in `cmd`) is built from the resolved slug/routes/runner/tls and marshaled.
- **`--json` implies detach for the tilt runner** (so the command returns to
  print the result rather than attaching to a foreground Tilt). The compose
  runner is already detached. (Same detach rule as `--wait`.)
- **Exit codes:** `0` on success including the already-running no-op; `1` on any
  error. Documented in the skill and README.

## 2. `--wait` readiness

`lane up --wait` brings the stack up, then **blocks until it's actually
reachable**, then returns (pairs with `--json` so the agent gets ready URLs).

- **Mechanism:** `internal/ready.WaitReady(urls []string, timeout time.Duration)
  error` — polls each routed URL (`http://<slug>.localhost/...`) every ~500ms via
  a short-timeout HTTP client; a URL is "ready" once it returns **any** HTTP
  status (i.e., the connection succeeds and isn't a Traefik 502/no-route).
  Returns nil when all routes are ready, or an error naming the route(s) that
  never came up by the deadline.
- **`--wait` implies detach** (so there's a running stack to poll): for the tilt
  runner it forces `-d`; the compose runner is already detached.
- `--wait-timeout` flag, default `90s`. On timeout → non-zero exit + the failing
  route.
- Pure/testable: `WaitReady` takes URLs + an injected `http.Client`; tested with
  `httptest` (a server that 502s N times then 200s).

## 3. Concurrency-safe parallel `up`

When several agents `lane up` at once, only the **shared** bring-up can race
(creating the `lane` network, starting `lane-proxy`). Per-stack work stays
parallel.

- `internal/lockfile.Acquire(path string, timeout time.Duration) (release func,
  err error)` — portable advisory lock via `os.OpenFile(O_CREATE|O_EXCL)` with
  spin-retry + a stale-lock age check; `release()` removes it. No new deps,
  works on all platforms.
- `proxy.Ensure` / `proxy.Up` acquire `~/.lane/proxy.lock` around
  `ensureNetwork` + the proxy `compose up`. The lock is held only for the few
  seconds of shared-infra setup, then released — stacks still come up
  concurrently.

## 4. Skill artifacts + marketplace packaging

Repo layout (a self-contained Claude plugin + a Cursor rule):

```
agent/
  claude/
    .claude-plugin/plugin.json        # name "lane", description, version, author
    skills/lane/SKILL.md              # the parallel-testing skill (frontmatter: name + when-to-use)
  cursor/
    lane.mdc                          # same recipe as a Cursor rule
.claude-plugin/marketplace.json       # repo-as-marketplace: lists the "lane" plugin (source: git-subdir → agent/claude)
```

- **Claude skill** (`SKILL.md`, frontmatter `name: lane`, `description: "Use when
  running or testing several services/worktrees in parallel — spins up isolated,
  port-conflict-free stacks"`). Body teaches the loop:
  1. (in a git worktree) `lane up --wait --json` → parse `urls`/`slug`.
  2. run tests/requests against the returned URL(s).
  3. `lane down` when done.
  Plus the exit-code/`--json` contract and the "each worktree = its own stack,
  no port conflicts" rationale.
- **Cursor rule** (`agent/cursor/lane.mdc`) — same recipe in Cursor's rule
  format; documented to copy into `.cursor/rules/`.
- **Marketplace** — `.claude-plugin/marketplace.json` makes the lane repo itself
  a marketplace listing the `lane` plugin (`source: git-subdir`, path
  `agent/claude`), so a user runs `/plugin marketplace add Dheeraj-Nalapat/lane`
  then installs `lane`. Exact manifest fields mirror the installed
  `claude-plugins-official` marketplace (verified: `$schema`, `name`,
  `description`, `owner`, `plugins[]` with `name`/`description`/`source`).
- **README** gets an "Using lane with coding agents (parallel testing)" section
  pointing at these + the `--json`/`--wait` contract.

## Files

```
internal/ready/ready.go            NEW — WaitReady(urls, timeout, client)
internal/ready/ready_test.go       NEW
internal/lockfile/lockfile.go      NEW — Acquire(path, timeout) → release
internal/lockfile/lockfile_test.go NEW
internal/proxy/proxy.go            Ensure/Up wrap shared bring-up in the lock
cmd/up.go                          --json, --wait, --wait-timeout; UpResult; --wait⇒detach
cmd/ls.go                          --json
agent/claude/.claude-plugin/plugin.json, agent/claude/skills/lane/SKILL.md   NEW
agent/cursor/lane.mdc              NEW
.claude-plugin/marketplace.json    NEW
README.md, CHANGELOG.md            agent section + entry
```

## Error handling

- `--wait` timeout → exit 1, stderr names the unready route; the stack is left
  running (agent can inspect / `lane down`).
- Lock acquisition timeout (another `up` holding it too long) → clear error
  ("another lane up is starting the proxy; retry"). Stale lock (older than ~30s,
  owner gone) is reclaimed.
- `--json` on error → non-zero exit, error to stderr, no partial JSON on stdout.

## Testing

- `ready.WaitReady` — httptest server returning 502 then 200 → returns nil;
  always-502 → times out with the URL named.
- `lockfile.Acquire` — second acquire blocks then fails within timeout while
  first holds; succeeds after release; stale lock reclaimed.
- `up --json` shape — build `UpResult` from a known slug/routes and assert the
  marshaled JSON fields (factor the result-building into a pure helper).
- `ls --json` — marshal a known `[]stack.Stack` → expected JSON.
- SKILL.md frontmatter present + valid; marketplace.json parses and references
  the plugin path.
- Live: two concurrent `lane up` (different slugs) don't error on proxy/network;
  `lane up --wait --json` returns only once `curl` of the URL succeeds.

## Backward compatibility

All additive: no flags ⇒ today's behavior. `--json`/`--wait` are opt-in; the
lock is invisible in the single-agent case (uncontended). Skill files are inert
unless an agent loads them.
