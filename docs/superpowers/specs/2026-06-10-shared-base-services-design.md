# Shared Base Services (Base-Borrowing) — Design

**Status:** approved (design).
**Date:** 2026-06-10
**Sub-project:** B of two. Builds directly on A (selective / minimal bring-up +
per-service reach), spec `2026-06-10-selective-service-bring-up-design.md`.

## Goal

When only a few services in a project have changed, run **just those** fresh in a
worktree and **borrow the rest** from a single already-running **base** stack
(typically the main checkout), instead of every worktree booting a full copy. This
saves machine resources and matches the workflow "I changed a couple of repos —
reuse everything else."

## Decisions (from brainstorming)

1. **Shared model:** a designated **base stack** (a full `lane up` from the main
   checkout). Worktrees run only their changed services and resolve everything else
   to the base.
2. **Trigger:** opt-in `--base` flag (boolean, auto-detect). The base is identified
   by **project name** (the manifest `name`), not slug.
3. **Naming rule:** a service named on the CLI is run **fresh/local**; an unnamed
   service is **borrowed** from the base. A service is never both.
4. **No local deps:** base mode does not auto-start dependencies (compose
   `--no-deps`); borrowed deps come from the base.
5. **Wiring mechanism (Approach 2):** per-borrowed-service `docker network connect`
   into the worktree's network with the service-name alias. (Rejected Approach 1, a
   shared flat network, because two fresh services that talk to each other — e.g.
   `web`+`api` — would resolve ambiguously against the base's copies.)

## 1. Scope & non-goals

**In scope:** `lane up <services...> --base` runs named services fresh (no local
deps) and connects each borrowed base container into the worktree network so the
fresh services resolve borrowed names to the base. Fresh services are auto-routed by
A (`<slug>-<service>.localhost`).

**Non-goals:** tilt-runner base mode (compose-runner only in v1, clear error for
tilt); cross-*project* borrowing (same project only); snapshotting/cloning the
base's data.

## 2. The model

- **Base** = a normal full `lane up` from the main checkout (slug == project name,
  e.g. `webapp`).
- **Worktree:** `lane up api --base` → `api` runs fresh on `webapp-featx_default`;
  `db`/`auth`/`web` are not started locally — their base containers are connected
  into `webapp-featx_default` with their service-name aliases.
- **Unambiguous resolution:** named = fresh/local, unnamed = borrowed; a service is
  never both, so fresh↔fresh resolves locally and borrowed names resolve to the
  base.

## 3. Bring-up

Base mode passes `--no-deps` so unnamed dependencies are not auto-started:

```
docker compose -p <slug> -f <base> -f <override> up -d --no-deps <named...>
```

(`--no-deps` is the escape hatch deferred from A, used internally by base mode.)

## 4. Base discovery

- `--base` is a boolean flag (auto-detect). The base is found by **project name**.
- A new `lane.project=<name>` label is added to every stack by `override.Generate`.
  Discovery: among running stacks, those with `lane.project=<name>` and slug ≠ ours
  are candidates; prefer the one whose slug == name (the canonical base).
- Errors:
  - our own slug == name (i.e. `--base` was run from the main checkout) →
    `this is the base stack; --base is for worktrees` (checked first).
  - none running → `no running base for "<name>"; start it from the main checkout with 'lane up'`.
  - multiple candidates → list them (explicit `--base <slug>` is a future add).
  - tilt runner → `base mode requires the compose runner`.

## 5. Wiring (connect / disconnect lifecycle)

- **Borrowed set** = base's running services − the named (fresh) set.
- **On `up`** (after `compose up`): for each borrowed base container,
  `docker network connect --alias <service> <slug>_default <base-container>`.
  The worktree network is compose's default network, `<slug>_default`.
- **DNS timing (caveat):** wiring happens immediately after `up`; a fresh service
  may boot a moment before its borrowed deps resolve. Most apps retry
  DB/dependency connections, so this is acceptable in v1 (the network is not
  pre-created). Documented.
- **On `down`** (before `compose down`): inspect `<slug>_default` and disconnect any
  connected container whose `com.docker.compose.project` ≠ our slug (the borrowed
  base containers), then run `compose down` (otherwise network removal fails with
  "active endpoints"). Base containers are only disconnected, never stopped.

## 6. Reporting (`--json`)

The up result gains base fields:

```json
{
  "slug": "webapp-featx",
  "base": "webapp",
  "fresh": ["api"],
  "borrowed": ["db", "auth", "web"],
  "urls": [
    { "service": "api", "url": "http://webapp-featx-api.localhost", "running": true }
  ]
}
```

Non-base runs omit `base`/`fresh`/`borrowed` (empty).

## 7. Code structure

- New `internal/basex/` — base discovery and borrowed-set computation (pure,
  testable; takes stack/container lists as input, returns base slug + borrowed
  names).
- `internal/dockerx/` — `ProjectContainers(slug)` (service→container names),
  `NetworkConnect(net, container, alias)`, `NetworkDisconnect(net, container)`,
  `ForeignContainers(net, ownSlug)` (parse `docker network inspect` for connected
  containers whose compose project ≠ ownSlug).
- `internal/override/override.go` — add the `lane.project=<name>` identity label.
- `cmd/up.go` — `--base` flag; base-mode branch (no-deps bring-up + post-up wiring);
  `base`/`fresh`/`borrowed` json fields.
- `cmd/down.go` — pre-`compose down` foreign-container disconnect.
- docs — a "Borrowing from a base stack" guide page.

## 8. Testing

Table-driven `_test.go` alongside sources.

- `basex`: discovery (match by name, exclude self, none → error, multiple →
  error/list, prefer slug==name); borrowed-set = base services − fresh.
- `dockerx`: parse a `docker network inspect` JSON fixture into foreign containers
  (project ≠ ownSlug); arg builders for `network connect --alias` and
  `network disconnect`.
- `override`: `lane.project` label present.
- e2e (throwaway, mirrors A's harness): base `lane up` (web + api + db); worktree
  `lane up api --base` → only the worktree `api` runs; `<wt>-api.localhost` → 200 and
  it reaches the base `db`; `lane down` disconnects the base container cleanly and
  the base stack keeps running; bringing the base down afterward leaves nothing
  orphaned.

## 9. Risks / caveats (documented)

- **Shared mutable data:** a borrowed `db` is the base's real database — worktree
  writes hit shared data. To run it fresh instead, name it: `lane up api db --base`.
  Schema-migrating changes should run the db fresh.
- **DNS timing** (§5) — apps must tolerate a brief window where a borrowed dep is not
  yet resolvable at boot.
- **Compose runner only** in v1 (tilt errors clearly).
- Assumes the project uses compose's default network (`<slug>_default`). Projects
  that define only custom networks and no default are unsupported in v1 (documented;
  detectable as a missing `<slug>_default` network → clear error).

## Open questions

- None blocking. Explicit `--base <slug>` (for multiple-candidate disambiguation)
  and tilt support are deliberate post-v1 follow-ups.
