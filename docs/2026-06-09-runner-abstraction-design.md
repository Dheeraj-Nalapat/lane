# lane — Runner Abstraction (drop the Tilt requirement) — Design Spec

**Date:** 2026-06-09
**Status:** Design complete — ready for implementation planning.
**Sub-project:** A of the generic-release effort (see "Context").

## Context

lane v1 works but is hard-tied to [Tilt](https://tilt.dev): every `lane up`
runs `tilt up -- --docker`, and every project must add a Tiltfile shim. That
blocks adoption by the large majority of projects that use plain
`docker compose` without Tilt.

This is **sub-project A** of generalizing lane for a public release. The full
effort decomposes into independent cycles:

- **A. Runner abstraction (this spec)** — drive Tilt *or* plain compose.
- B. HTTPS/TLS for `*.localhost`.
- C. Robustness & UX (Docker-down, `:80` taken, idempotency, image-tag isolation…).
- D. Release engineering (CI, LICENSE, real releases, Homebrew tap, semver).
- E. External docs (zero-to-running, per-framework recipes).

Order: A → D (parallel) → B → C → E. Each is its own spec → plan → build.

## Goal

Let lane run **any docker-compose project with zero Tiltfile changes**, while
keeping the full Tilt experience (live-reload, dashboard) for projects that use
Tilt. Selection is automatic.

## Non-goals (this sub-project)

- Live-reload for compose projects (no `docker compose watch` yet — a possible
  future). Compose stacks rebuild on `lane up --build`.
- HTTPS, k8s, or any backend beyond Tilt + Compose.
- Changing the proxy, slug logic, override generation, or `down`/`ls`/`open`
  beyond the tiny guards noted below.

## Approach (chosen)

A `Runner` interface with two implementations — `tiltRunner` (today's behavior)
and `composeRunner` (new) — selected automatically. Everything else stays
shared. The compose path is *simpler and more robust*: lane sets `-p <slug>`
directly, so the "Tilt ignores `COMPOSE_PROJECT_NAME`" problem cannot occur, and
the override is applied with `-f` instead of via env vars.

(Rejected: unifying on `docker compose watch` and dropping Tilt — discards the
Tilt dashboard + mature live_update; a pluggable N-driver registry — YAGNI.)

## Architecture

### New package `internal/runner`

```go
// RunSpec is everything a runner needs to bring one stack up.
type RunSpec struct {
    Slug         string
    Dir          string         // project directory
    ComposePath  string         // absolute path to base compose
    OverridePath string         // absolute path to generated override
    Routes       []override.Route
    Detach       bool           // -d
    Build        bool           // --build: force image rebuild
    TiltPort     int            // 0 when not Tilt
    DynamicPath  string         // tilt-UI route file path; "" when not Tilt
    Env          []string       // base env (compose project name etc.)
}

// Runner brings a stack up. Teardown stays shared (cmd/down.go).
type Runner interface {
    Up(RunSpec) error
    Name() string // "tilt" | "compose"
}
```

### Selection (in `cmd/up.go`)

First match wins:
1. `.lane.toml` `runner = "tilt" | "compose"` if set.
2. A `Tiltfile` exists in the project dir → `tiltRunner`.
3. Otherwise → `composeRunner`.

A small pure function makes this unit-testable:
```go
// Select returns "tilt" or "compose".
func Select(manifestRunner string, tiltfilePresent bool) string
```

`up` performs the shared prep regardless of runner: resolve slug → collision
check → `compose.ServiceNames` → `override.Generate` → write override → ensure
proxy. It then allocates a Tilt port and writes the Tilt-UI dynamic route
**only when the tilt runner is selected**, and hands a populated `RunSpec` to
the chosen runner.

### `tiltRunner` (behavior unchanged, moved behind the interface)

- Env: `COMPOSE_PROJECT_NAME`, `LANE`, `LANE_SLUG`, `LANE_COMPOSE_OVERRIDE`,
  optional `LANE_API_TARGET`.
- Runs `tilt up --host 0.0.0.0 --port <TiltPort> -- --docker` in `Dir`.
- Foreground by default; `--detach` backgrounds it and writes
  `~/.lane/run/<slug>.pid`.
- Requires the project's one-time Tiltfile shim (accept `--docker`, append
  `LANE_COMPOSE_OVERRIDE`, `project_name=LANE_SLUG`). Keeps live-reload + the
  Tilt dashboard routed at `tilt-<slug>.localhost`.

### `composeRunner` (new)

Builds and runs:
```
docker compose -p <Slug> -f <ComposePath> -f <OverridePath> up -d [--build]
```
- **Detached by default** (native `up -d`); prints the app URLs and returns.
- **No** Tiltfile, shim, `LANE_*` env, Tilt port, dynamic route, or pidfile.
- lane owns the project name via `-p <Slug>`; the override is applied directly
  via `-f`.
- `--build` forces a rebuild; otherwise Compose builds only missing images.

Command construction is a pure, testable function:
```go
func buildComposeArgs(slug, composePath, overridePath string, build bool) []string
// → ["compose","-p",slug,"-f",composePath,"-f",overridePath,"up","-d", ("--build")?]
```

## Shared / unchanged

- **`down`** — already runs `docker compose -p <slug> -f base -f override down`;
  runner-agnostic, no change.
- **`ls`, `open`, proxy, slug, doctor** — unchanged.
- **Override generator** — one small change: **omit the `lane.tilt.port` label
  when `TiltPort == 0`** (compose stacks). Everything else (`!reset`
  ports/container_name, Traefik routing labels, identity labels, network) is
  identical for both runners.
- **`view` / `ls`** — render the Tilt row/column only when `lane.tilt.port > 0`;
  compose stacks show just their app route(s).

## Manifest change

Add an optional field to `.lane.toml`:
```toml
runner = "compose"   # "tilt" | "compose"; omit for auto-detect
```
Validation: if present, must be `tilt` or `compose`. Default `""` (auto).

## CLI change

- `lane up` gains `--build` (forces rebuild; passed to the compose runner;
  ignored by the tilt runner with a note, since Tilt manages its own builds).
- `-d/--detach` keeps meaning for the tilt runner; for the compose runner the
  stack is already detached, so `-d` is a no-op (documented).

## Error handling

- `runner = "tilt"` but no Tiltfile → warn (proceed; the user may generate one).
- `runner = "compose"` (or auto→compose) but `compose_file` missing/unreadable →
  fail with a clear message.
- Invalid `runner` value → manifest load error naming the allowed values.

## Testing

- **`runner.Select`** — table test: manifest override beats detection; Tiltfile
  presence selects tilt; neither → compose.
- **`buildComposeArgs`** — exact arg slice with/without `--build`.
- **Existing unit suite** stays green (the move behind the interface must not
  change `tiltRunner` behavior; assert `UpArgs` unchanged).
- **Live smoke (manual):** the synthetic two-stack test gains a **Tilt-less**
  variant — a project with only a `docker-compose.yml` + `.lane.toml` and **no
  Tiltfile** — proving `lane up` brings it up behind the proxy with zero shim,
  reachable at `<slug>.localhost`, and `lane down` cleans it.

## Rollout / impact

- Fully backward compatible: existing Tilt projects (ReMind) keep working —
  detection finds their Tiltfile and uses `tiltRunner`.
- New value: any compose project works immediately with just a `.lane.toml`
  (which `lane init` already scaffolds) and no Tiltfile edits.
- Docs (sub-project E) will document both paths; this spec only requires
  updating the onboarding section to note the Tiltfile shim is **Tilt-only**.
