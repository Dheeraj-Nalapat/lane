# lane — Robustness & UX (C1) — Design Spec

**Date:** 2026-06-09
**Status:** Design complete — ready for implementation planning.
**Sub-project:** C1 of the generic-release effort (C split into C1 hardening +
C2 interactive view).

## Context

lane's features are in place (runners, release hygiene, HTTPS). This pass
hardens day-to-day use: clearer errors, safe re-runs, a `restart`, optional
volume cleanup, and per-slug image isolation. The interactive `lane view`
control panel is **C2** (separate spec, built on C1's `restart`/`down`).

## Goal

Make lane fail clearly instead of cryptically, behave predictably on re-run, and
keep each stack's built images isolated — without changing the happy path.

## Scope (4 items)

1. Preflight error handling.
2. Idempotent `up` + `lane restart`.
3. `lane down --volumes`.
4. Per-slug image-tag isolation.

Non-goals: the interactive view (C2); anything not listed above.

## 1. Preflight error handling

New package **`internal/preflight`** with small, independently-testable checks,
used by both `doctor` (informational) and the action commands (gating).

- **`DockerRunning() error`** — `docker info`; non-nil error ⇒ "Docker doesn't
  appear to be running — start Docker and retry."
- **`ComposeOK() (bool, string)`** — runs `docker compose version`, returns
  (ok, versionLine). Internally uses an unexported pure
  `composeOK(line string) bool` (the version-compare logic **moves here from
  `internal/doctor`**). `doctor` calls `preflight.ComposeOK()` (no duplicated
  parsing); the pure `composeOK` is unit-tested in-package. Below 2.20, action
  callers stop before generating a `!reset` override.
- **Port conflicts** — pre-binding `net.Listen(":80")` is unreliable (non-root
  can't bind `:80`, yielding false positives). The robust signal is Docker's own
  error: `proxy.Up` captures `docker compose up` stderr and, when it contains
  `address already in use` or `port is already allocated` **and** `lane-proxy`
  is not the owner, returns: "port 80 is in use by another process — free it (or
  stop that service), then `lane proxy up`." A helper
  `preflight.IsPortConflict(stderr string) bool` makes this testable.

Wiring: `cmd/up.go`, `cmd/proxy.go` (`up`), and `cmd/tls.go` (`enable`) call
`preflight.DockerRunning()` and `preflight.ComposeOK()` up front and return the
actionable error. `doctor` keeps its current output but sources the checks from
`preflight`.

## 2. Idempotent `up` + `restart`

- **`up` already-running guard.** `cmd/up.go` already calls
  `dockerx.SlugOwner(sl)`. Extend it: if the owner path **equals this dir**, the
  stack is already up — print "stack `<slug>` already running — use `lane
  restart` to recreate, or `lane down` to stop" and exit 0 (no second Tilt
  process, no duplicate). The existing different-path collision error is
  unchanged. *(Chosen: no-op when running, rather than silently converging, so
  re-running `up` is always safe and explicit.)*
- **`lane restart [path]`** — new command in `cmd/restart.go`: run the existing
  `runDown` then `runUp` for the same dir/args. Reuses both; no new lifecycle
  logic. This is the supported way to apply `.lane.toml`/compose changes.

## 3. `lane down --volumes`

Add a `--volumes` bool flag to `down` (no `-v` shorthand — global `-v` is
verbose). When set, append `--volumes` to the `docker compose ... down` args so
named volumes are removed (data reset). Default false ⇒ today's behavior
(volumes preserved). Reflect it in the teardown message ("… and volumes").

## 4. Per-slug image-tag isolation

**Compose runner — automatic, generic.** lane's `internal/compose` reader gains
`BuiltServices(path) ([]string, error)` returning services that declare a
`build:` section. In `cmd/up.go`, those names are passed to `override.Spec`
(new field `BuiltServices []string`); `override.Generate`, for each such service,
emits `image: !reset null`. Compose then names the built image by project
(`<slug>-<service>`), so each stack/worktree builds its own image — no clobber.
Pulled-only services (e.g. `redis:7`, no `build:`) are untouched.

> Why `!reset` the image: a built service that also pins `image: foo/bar` would
> otherwise share that fixed tag across stacks. Resetting it lets Compose
> fall back to the project-prefixed name, which is already slug-unique.

**Tilt runner — documented pattern.** Image refs live in the Tiltfile, which
lane never rewrites. lane already exports `LANE_SLUG`; onboarding docs and
ReMind's Tiltfile enable the existing hook:
```python
tag = (":" + lane_slug) if lane_slug else ""
docker_build("remind/platform-server" + tag, ...)
```
So built images become `remind/platform-server:<slug>`. Documented, opt-in; not
automatic (lane can't know a project's `docker_build` refs).

## Files

```
internal/preflight/preflight.go      NEW — DockerRunning, ComposeOK, composeOK, IsPortConflict
internal/preflight/preflight_test.go NEW
internal/doctor/doctor.go            call preflight.ComposeOK() (remove local composeOK)
internal/compose/compose.go          + BuiltServices()
internal/override/override.go        + Spec.BuiltServices; image: !reset null for them
internal/proxy/proxy.go              translate port-conflict stderr in Up()
cmd/up.go                            preflight calls; already-running no-op; pass BuiltServices
cmd/proxy.go, cmd/tls.go             preflight calls
cmd/restart.go                       NEW — lane restart
cmd/down.go                          + --volumes flag
docs/onboarding-remind.md, README.md image-isolation pattern note
```

## Error handling

Every gating check returns a single, actionable sentence (what's wrong + the fix
command). No stack traces. Preflight runs before any side effect (no
half-written override/cert on a failed preflight).

## Testing

Pure unit tests:
- `preflight.composeOK` (version table — moved with its existing cases).
- `preflight.IsPortConflict` — true for "address already in use" / "port is
  already allocated"; false otherwise.
- `compose.BuiltServices` — fixture compose with one `build:` service + one
  pulled service → returns only the built one.
- `override.Generate` with `BuiltServices:["server"]` → `server` has
  `image: !reset null`; a non-built service does not; `BuiltServices:nil` →
  no image reset (backward compatible).

Integration/manual:
- `lane up` twice in a row → second prints "already running", exits 0, no
  duplicate Tilt/containers.
- `lane restart` → stack recreated.
- `lane down --volumes` → named volume gone (`docker volume ls`).
- Compose project with a built+`image:` service across two slugs → two distinct
  images (`docker images`).
- Stop Docker → `lane up` prints the daemon message, non-zero exit.

## Backward compatibility

Happy path unchanged: preflight passes silently when healthy; `BuiltServices`
empty ⇒ identical override; `--volumes` defaults off; `up` only no-ops when a
same-path stack is already running.
