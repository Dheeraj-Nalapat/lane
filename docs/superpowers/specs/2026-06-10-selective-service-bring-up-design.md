# Selective / Minimal Bring-Up + Per-Service Reach — Design

**Status:** approved (design); CLI surface in §2 accepted as the *recommended* variant — open to veto at review.
**Date:** 2026-06-10
**Sub-project:** A of two. B (shared common services) builds on this and has its own spec.

## Goal

Let a developer or agent bring up a **subset** of a project's services (a single
service, an ad-hoc list, or a compose profile) instead of the whole stack, with
dependencies pulled in automatically — and make **every** HTTP service reachable
individually at a stable URL with no extra config.

This serves the core intent: agents spin up the minimal slice they need to test a
change, in parallel, using fewer machine resources.

## Decisions (from brainstorming)

1. **Dependencies:** when a subset is selected, `depends_on` is auto-included
   recursively (default). "Minimal" = named services + what they need to run.
   A `--no-deps` escape hatch is explicitly deferred.
2. **Selection mechanism:** ad-hoc **service names** + Docker Compose **profiles**.
   No lane-specific "groups" concept — profiles live in the user's compose file.
3. **Per-service reach:** auto-route **every** HTTP service (in addition to explicit
   `[[routes]]`), so any service is reachable by name with zero config.
4. **Hostname format:** dashed, single-level **`<slug>-<service>.localhost`** — stays
   within the existing `*.localhost` wildcard TLS cert and matches the worktree
   naming style (`webapp-featx.localhost`).
5. **Port discovery:** layered, skip-what's-unknowable (see §5).

## 1. Scope & non-goals

**In scope:** subset bring-up by name and/or profile (deps auto-included);
auto-routing at `<slug>-<service>.localhost`; reporting/waiting only on services
that are actually running.

**Non-goals:** shared cross-stack services (B); non-HTTP (TCP/gRPC) routing from
the host; `--no-deps`.

## 2. CLI surface

```
lane up [services...]            # subset by name (deps auto-included)
lane up --profile minimal        # activate a compose profile (repeatable)
lane up api --profile debug      # combine names + profile
lane up                          # whole stack (unchanged)
```

- **Positional args are service names** (mirrors `docker compose up <svc>`).
- The project directory moves from the `[path]` positional to a **`-C, --path`**
  flag (default: cwd), applied consistently across `up`, `down`, `logs`,
  `restart`, `open` for uniformity.
- `-p, --profile` is a repeatable string flag.

**Breaking change (accepted, pre-1.0):** the `[path]` positional on `up` (and the
other commands listed) is replaced by `-C/--path`. Rejected alternative: keep
`[path]` positional and select services via repeatable `-s/--service`. The
positional-services form was chosen for compose-like ergonomics.

## 3. Config (`.lane.toml`) changes

- **`[[routes]]` becomes optional** (currently required). Auto-routing covers the
  common case; an explicit route is only needed to override a host or to route a
  service whose port can't be auto-discovered.
- New optional block:

```toml
[autoroute]
enabled = true            # default true; false disables auto-routing entirely
exclude = ["worker"]      # services that are never auto-routed
```

Profiles are not represented in `.lane.toml` — they are read from the compose file
by Docker/Tilt directly.

## 4. Service selection → runners

`runner.RunSpec` gains `Services []string` and `Profiles []string`.

**compose runner** — native; dependency expansion is free:
```
docker compose [--profile P]... -p <slug> -f <base> -f <override> up -d [--build] [services...]
```
Global flags (`--profile`, `-p`, `-f`) precede `up`; service names follow it.

**tilt runner** — selected resources are passed to `tilt up <resource...>` (Tilt
resource names equal compose service names; Tilt brings their dependencies up as it
loads the compose config). Resources are inserted **before** the `--` separator in
`tiltx.UpArgs` (Tilt flags must precede `--`). Profiles are activated by exporting
`COMPOSE_PROFILES` in the runner env, consumed by the Tiltfile shim's
`docker_compose(..., profiles=...)`.

**Design risk (validate early):** Tilt profile activation is less direct than
compose's. If `COMPOSE_PROFILES` + shim does not cleanly activate profiles under
Tilt, profiles become **compose-runner-only** for v1; service-name selection still
works for both runners. This is a documented limitation, not a blocker.

## 5. Auto-routing & port discovery

A new `compose.Services(path)` returns, per service: name, whether `build:` is
present, and a discovered container port via the layered rule:

1. If the service has an explicit `[[routes]]` entry → use that port (always wins).
2. Else `expose:` → if it lists **exactly one** port, use it.
3. Else `ports:` container-side `target` (read **before** lane `!reset`s host
   publishing) → if **exactly one**, use it.
4. Else **skip** the service and log:
   `skipped <svc> (no single exposed port — add a [[routes]] entry to route it)`.

The parser handles compose short syntax (`"8000:80"`, `"80"`) and long syntax
(`{target: 80, published: 8000}`).

**Resolved route set** = explicit routes ∪ auto-routes, with **one hostname per
service**: the explicit route's host if declared, otherwise
`<slug>-<service>.localhost`. Excluded services (`[autoroute].exclude`) and
port-unknown services get no route. Auto-routing applies to *every* eligible
service regardless of whether it is in the current subset — see §6 for why that is
safe.

Note: the bare `<slug>.localhost` host is only produced by an **explicit**
`[[routes]]` entry (whose host defaults to `{slug}`). Auto-routing always uses the
dashed `<slug>-<service>.localhost` form. The default `lane init` scaffold keeps an
explicit primary route, so `webapp.localhost` continues to work out of the box;
routes being optional is for projects that are happy with the dashed per-service
hosts.

## 6. Override / Traefik changes

`override.Generate` already emits one Traefik router per routed service
(`<slug>-<service>`) and attaches the `lane` network + Traefik labels only to routed
services. We pass it the **merged** route set so it labels all routable services.

**Key safety property:** Traefik discovers and routes only **running containers**.
Labeling a service that is not part of the active subset is harmless — there is no
container for Traefik to route to, so the advertised host simply 502s until/unless
that service is started. This lets us label statically (from the compose file) and
avoid replicating compose's dependency/profile expansion. No change to the
`!reset` or TLS-router logic.

## 7. Reporting: `ls` / `view` / `--json` / `--wait` (running-aware)

To report accurately without re-implementing compose's selection logic, lane asks
Docker what is actually running:

- New `dockerx.RunningServices(slug)` lists containers labeled
  `com.docker.compose.project=<slug>` and reads `com.docker.compose.service`.
- `--json`, `ls`, and `view` show each route with a status: **running** vs
  **declared-but-not-running**. Auto-routed URLs are included.
- `--wait` waits only on routes whose service is **running**, so an agent waiting
  on `webapp-api.localhost` is not blocked by services it did not start.

## 8. Error handling

- Unknown service name (not in the compose file) → fail fast:
  `no such service "foo" (have: api, web, db)`.
- Unknown profile → not pre-validated; compose's own error surfaces (compose owns
  profiles).
- Subset resolves to no routable HTTP service → the stack still comes up; warn that
  nothing was routed.

## 9. Testing

Table-driven `_test.go` files alongside sources, matching the existing style.

- `compose`: port discovery across expose / ports (short + long) / none / multiple /
  build-present.
- routing merge: explicit wins; auto fills the rest; `exclude` honored; host format.
- compose runner: arg order with profiles + services + `--build`.
- `tiltx`: resources placed before `--`; `COMPOSE_PROFILES` env set.
- `manifest`: `[[routes]]` optional; `[autoroute]` parsing and defaults.
- `dockerx.RunningServices`: parse a `docker ps` JSON fixture.
- Integration (extends the existing whoami-style harness): `lane up <one-svc>` →
  only that service + its deps run → its `<slug>-<svc>.localhost` returns 200; a
  non-started service's route is reported not-running.

## 10. Files touched

- `cmd/up.go` — selection flags, `-C/--path`, merged routes, running-aware wait/json.
- `cmd/{down,logs,restart,open}.go` — `-C/--path` consistency.
- `internal/compose/compose.go` — `Services` + port discovery.
- `internal/manifest/manifest.go` — optional routes, `[autoroute]`.
- `internal/override/override.go` — consume merged routes (minimal/no change).
- `internal/runner/{runner,compose,tilt}.go`, `internal/tiltx/tiltx.go` —
  services/profiles plumbing.
- `internal/dockerx` — `RunningServices`.
- `internal/ls` / `internal/ui` (wherever `ls`/`view` render) — running status.
- docs — new "Selecting & reaching services" guide page.

## Open questions

- None blocking. The Tilt profile risk (§4) is resolved during implementation; if it
  fails, profiles ship compose-only with a documented note.
