# lane — Design Spec (v1.0)

**Date:** 2026-06-08
**Status:** Design complete — ready for implementation planning.

> **Name:** *lane* — a place a ship docks. Every stack gets its own lane
> instead of fighting over a shared port slot. CLI verbs: `lane up`,
> `lane ls`, `lane down`.

## Problem

Developing many projects at once — and the *same* project across multiple git
worktrees — causes constant **host-port collisions**. Services are brought up
with Tilt, but ports are hardcoded throughout (e.g. ReMind: platform `:8000`,
agent `:8100`, Vite `:5173`, plus docker-compose `8000:8000`/`8100:8100`
mappings, the Vite proxy target `localhost:8000`, and Tilt's own UI on `:10350`).
Every one of those collides the moment a second stack starts.

The user needs to run multiple stacks **simultaneously**, each **reachable in a
browser at the same time**, with **friendly, distinguishable URLs**.

## Goals

- Run any number of stacks (different projects, or one project across worktrees)
  at once with **zero host-port conflicts**.
- Each stack reachable at a stable, friendly hostname (e.g. `remind.localhost`,
  `remind-featx.localhost`).
- A **reusable convention** plus a publishable CLI tool (`lane`), not a
  one-off hack.
- **Non-invasive**: using lane must never change behavior in
  production/CI/normal dev, and must never mutate committed project files.

## Non-goals (v1)

- Supporting arbitrary third-party projects that don't follow the convention.
  v1 targets the user's own projects, built at publishable quality; generalize
  later (YAGNI).
- Local-process (non-Docker) multi-stack mode. v1 standardizes on Docker.
- HTTPS/TLS for local hostnames (HTTP `:80` only in v1; note as future).

## Decisions made

| Dimension        | Decision |
|------------------|----------|
| Access pattern   | Multiple UIs reachable in the browser **at the same time** |
| Scope            | A **reusable convention** across all Tilt projects, my projects first |
| Addressing       | **Friendly hostnames** via one shared reverse proxy |
| Run mode         | **Standardize on Docker** (Tilt `--docker` / compose path) |
| Packaging        | **`lane` CLI** as the cockpit over a Traefik + slug mechanism |
| v1 audience      | My own projects first, publishable-quality, not yet general-purpose |
| Override style   | lane **generates** the proxy/network/port wiring; committed files stay pristine |
| Language         | **Go** — single static binary, zero runtime deps; users get a dependency-free artifact regardless of their own stack (TS/Java/etc.) |

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Shared, always-on:  Traefik  (network: lane)            │
│  • owns host :80                                           │
│  • watches the Docker socket, auto-discovers labeled       │
│    containers                                              │
│  • routes Host(<slug>.localhost) → that container's port   │
└───────────────▲─────────────────────▲─────────────────────┘
                │ lane network        │ lane network
     ┌──────────┴─────────┐  ┌─────────┴──────────┐
     │ Stack: remind      │  │ Stack: remind-featx │   ← same repo,
     │ COMPOSE_PROJECT=…  │  │ COMPOSE_PROJECT=…   │     two worktrees
     │ ui, server, agent  │  │ ui, server, agent   │
     │ (no host ports)    │  │ (no host ports)     │
     └────────────────────┘  └─────────────────────┘
                ▲                       ▲
                └────────── lane CLI ───┘
        (derives slug, sets env, runs tilt, lists / tears down)
```

**Core principle — lane is service-agnostic.** lane never needs to know what
a service *is*. It routes whatever the manifest names (static build, dev server,
anything), strips host ports, resets container names, and joins the network. The
same logic works identically for an `nginx:80` static build and a `vite:5173`
dev server. Anything frontend-framework-specific (e.g. Vite HMR) is an
**opt-in project choice**, never part of lane's engine.

Three cleanly separated pieces:

1. **Traefik (shared infra).** Started once, owns host `:80` and the external
   `lane` Docker network. Watches the Docker socket and routes by hostname.
   Knows nothing about any project.

2. **Each stack (a normal compose project).** Must, *via generated overlay*:
   join the `lane` network, publish **no host ports**, and carry Traefik
   labels naming its hostname. Inter-service calls stay on the stack's own
   private compose network (`agent-server:8100`), unchanged from today.

3. **lane CLI (the cockpit).** Derives the per-stack `SLUG`, exports
   `COMPOSE_PROJECT_NAME` + a free Tilt UI port + a per-slug image-tag
   namespace, runs `tilt up -- --docker`, and reads Docker labels back for
   `ls`/`down`. **Holds no state of its own** — truth lives in Docker, so
   nothing drifts.

### Why no host ports = no conflicts

`ports: ["8000:8000"]` binds container port 8000 to **host** port 8000 — a
finite, machine-wide resource only one process can own. That is the root cause
of the collisions. If stacks don't publish host ports, the container still
listens internally (reachable inside Docker networks) but nothing touches the
host. Traefik reaches containers **over the `lane` network**, not via host
ports, and fans them out by hostname. Only Traefik owns a host port (`:80`).

## The per-project contract

Split into what's **committed** vs **generated** (the split is what keeps churn
near zero and guarantees non-invasiveness):

**Committed to the repo (small, one-time):**
- A `.lane.toml` manifest: project base-name, and which service(s) are web
  entrypoints + their internal ports. **This is the only required change.**
- *(Optional, only if you want frontend hot-reload)* the dev-server tweaks
  described in "Optional: frontend hot-reload" below. Not needed for static-build
  UIs.

**Generated by lane at `up` time (never committed):**
- A compose override (`*.lane.override.yml`) that:
  - attaches services to the `lane` network,
  - adds Traefik router/service labels with `Host(<slug>.localhost)`,
  - **strips host-port publishing** from the base compose using the
    `ports: !reset []` YAML tag (Docker Compose ≥ 2.20) — so published ports in
    the committed file are neutralized **without editing it**. A service that
    genuinely needs a direct host port can instead be re-published as an
    ephemeral loopback port (`127.0.0.1::<port>`) and reported.
  - **resets any hardcoded `container_name`** (`container_name: !reset null`) so
    Compose auto-names containers with the project prefix — otherwise two stacks
    of the same repo collide on a fixed container name.
- Environment: `COMPOSE_PROJECT_NAME=<slug>`, `LANE_SLUG=<slug>`, a free Tilt
  UI port, a per-slug image-tag namespace.

> **Tilt integration note (verified live, 2026-06-09):** Tilt does **not** honor
> `COMPOSE_PROJECT_NAME`; it defaults the compose project to the Tiltfile's
> directory name. The Tiltfile shim must therefore pass
> `docker_compose(..., project_name=os.getenv("LANE_SLUG"))` so isolation and
> `lane down` key off the slug. The shim must also `config.define_bool("docker")`
> so lane's `-- --docker` is accepted, and lane invokes Tilt with
> `--host 0.0.0.0` so the Tilt UI is reachable from the proxy via
> `host.docker.internal`.

## Isolation guarantees

- **Non-invasive / production-safe.** lane never mutates committed files.
  All wiring lives in a generated override + environment that exist only during
  a `lane up`. Committed compose, Dockerfiles, and app code are untouched.
  It rides Tilt's existing dev-only `--docker` path. Vite tweaks are env-gated
  no-ops in normal `npm run dev`. Production deploys never run this. Worst case,
  `lane down` leaves the project byte-for-byte unchanged.
- **Worktree independence.** Each worktree runs its **own** `tilt up` process,
  own Tilt UI port, and own compose project. Tilt only watches its own
  worktree's files, so editing worktree A triggers rebuild/restart **only in
  A** — B never sees it. To make isolation total, lane **namespaces built
  image tags by slug** (compose already prefixes volumes/networks by project
  name), so two stacks share nothing mutable and a rebuild in one never clobbers
  the other's image.

## Slug derivation

The **slug** is the per-stack identity. Everything else derives from it:
`Host(<slug>.localhost)`, `COMPOSE_PROJECT_NAME=<slug>`, the per-slug image-tag
namespace, and the Tilt UI port. Requirements: **deterministic** (same checkout
→ same slug every run, so URLs are bookmarkable), **unique** across projects and
worktrees, and **DNS/Docker-safe**.

### Resolution ladder (first match wins)

1. `--slug <x>` flag — explicit override, always wins.
2. `LANE_SLUG` env var — same, for scripting.
3. `.lane.toml` `name` (base) **+ auto worktree suffix** — the normal path.
4. Fallback: sanitized directory basename (so `lane up` still works in an
   un-onboarded project; less pretty).

### Normal path (worktree-aware)

The committed `.lane.toml` carries the **base name** (`name = "remind"`).
Because it's committed, every worktree inherits the same base. lane then
detects whether it is in a **linked git worktree** and appends a suffix:

- **Main checkout** → slug = `remind`
- **Linked worktree** → slug = `remind-<worktree>` → e.g. `remind-featx`

Detection: compare `git rev-parse --git-dir` vs `--git-common-dir`. If they
differ, it's a linked worktree; the worktree's name is the `.git/worktrees/<name>`
leaf (the same name `git worktree list` shows).

### Suffix source: git worktree name (not branch)

| Source | Stable? | Problem |
|---|---|---|
| **git worktree name** (chosen) | yes — fixed at `git worktree add` time | none; tied to the physical checkout |
| branch name | no — changes on checkout/rename | switching branches silently changes the slug → URL moves, orphaned containers |
| directory basename | yes | breaks if two worktrees in different parents share a leaf name; redundant with base |

Decision: **git worktree name.** Branch-independent, stable, matches
`git worktree list`.

### Sanitization & collision handling

- Lowercase; map non-`[a-z0-9-]` → `-`; collapse repeats; trim leading/trailing
  `-`; must start alphanumeric; cap length (~40 chars, under the 63-char
  DNS-label limit).
- Truth lives in Docker labels, so `lane up` checks whether the resolved slug
  is **already claimed by a stack from a different path** and refuses with a
  clear message rather than silently colliding.

## CLI command surface

### Core lifecycle

| Command | Behavior |
|---|---|
| `lane up [path]` | Derive slug → ensure proxy is up → generate the override → set env (`COMPOSE_PROJECT_NAME`, free Tilt port, image-tag namespace) → run `tilt up -- --docker`. Prints the friendly URL(s). **Foreground by default**; `-d/--detach` backgrounds it. |
| `lane down [path]` | Tear down this stack (`tilt down` in the reconstructed context) and delete the generated override. Repo left pristine. |
| `lane ls` | Quick, flat, scriptable list of running stacks: slug, project path, URL(s), Tilt port, status. |
| `lane proxy up\|down\|status\|logs` | Manage the shared Traefik. `up` auto-runs it when needed; explicit control still available. |
| `lane doctor` | Preflight: Docker running? Compose ≥ 2.20 (for `!reset`)? `lane` network present? `*.localhost` → 127.0.0.1? Tilt installed? `:80` free or owned by our Traefik? Reports actionable fixes. |

### Quality-of-life (all in v1)

| Command | Behavior |
|---|---|
| `lane init` | Scaffold `.lane.toml` by inspecting the project's compose; propose the web entrypoint service + port. ~30-second onboarding per project. |
| `lane open [--slug]` | Open the stack's URL in the browser. |
| `lane logs [--slug]` | Tail a detached stack's Tilt/service logs without opening the Tilt UI. Pairs with `-d`. |
| `lane view [--watch]` | Rich terminal **control panel**: a tree of each slug → hostname/URL → the services Traefik is actually routing to it (internal port, health) → Tilt port. Built from **Docker labels + the Traefik API** (`/api/http/routers`, `/api/http/services`), so it shows real routing, not just intent. Static snapshot by default; `--watch` live-refreshes. Rendered with a Go TUI lib (Bubble Tea / lipgloss). |

`ls` = quick/scriptable; `view` = rich/human. They serve different needs.

### Global flags

- `--slug <x>` — override identity (top of the resolution ladder).
- `--dry-run` — print the generated override + the exact `tilt`/env it *would*
  run, then exit. Directly backs the non-invasive requirement: see precisely
  what lane does before it acts.
- `-v/--verbose`.

### State model (stateless)

Truth is derived from **Docker labels**, not a state file — nothing drifts.
Every managed container carries:

```
lane.managed=true        lane.slug=<slug>
lane.project.path=<abs>  lane.url=http://<slug>.localhost
lane.tilt.port=<port>
```

`lane ls`/`view` read these (plus the Traefik API); `lane down` reconstructs
context from `lane.project.path`. The **only** runtime state is a pidfile under
`~/.lane/run/<slug>.pid`, created **only** when `-d/--detach` is used, to stop
the host-side Tilt process later.

## Tilt UI routing

The Tilt dashboard is routed through Traefik at **`tilt-<slug>.localhost`** —
friendly URLs instead of memorizing per-stack Tilt ports.

Implementation note: Tilt's UI is a **host process**, not a container, so
Traefik can't discover it by container label. lane writes a small Traefik
**file-provider** dynamic config mapping `Host(tilt-<slug>.localhost)` →
`http://host.docker.internal:<tilt-port>`. Traefik is launched with
`host-gateway` (`--add-host=host.docker.internal:host-gateway` on Linux) so it
can reach the host. A free host port is still allocated for Tilt to bind; the
hostname is just the friendly front door.

## Optional: frontend hot-reload (dev server behind the proxy)

**Opt-in, not required.** lane routes whatever the manifest names. If a
project's UI service is a **static build** (like ReMind's `--docker` `ui`
service, nginx on `:80`), lane just routes it — nothing to configure, no HMR.

This section applies **only if a project chooses to run a Vite dev server in a
container** behind the proxy to get hot-reload. To do that, the project's compose
runs the UI service as `npm run dev` with source mounted, and applies the
env-gated `vite.config.ts` block below. Vite's dev server has three behaviors
that break behind a proxy on a custom hostname; all are handled by the single
gated block, so normal `npm run dev` is byte-for-byte unaffected:

| Problem | Cause | Fix |
|---|---|---|
| "Blocked request" | Vite rejects unrecognized Host headers (anti-hijack) | `server.allowedHosts: ['.localhost']` |
| HMR (hot reload) dies | Page loads on `:80` via Traefik, but Vite's reload WebSocket phones home on the wrong port | `server.hmr.clientPort: 80` + `server.host: '0.0.0.0'` |
| `/api` 404s | Proxy target `localhost:8000` is wrong inside Docker; backend is the `server` container | env-driven `target` |

Committed change (the **only** app-code edit, fully gated):

```ts
const lane = !!process.env.LANE
export default defineConfig({
  server: {
    host: lane ? '0.0.0.0' : 'localhost',
    allowedHosts: lane ? ['.localhost'] : undefined,
    hmr: lane ? { clientPort: 80 } : undefined,
    proxy: {
      '/api': {
        target: process.env.LANE_API_TARGET || 'http://localhost:8000',
        changeOrigin: true,
      },
    },
  },
})
```

lane sets `LANE=1` and `LANE_API_TARGET=http://<backend-service>:<port>` at
run time. Outside lane nothing activates. Other frontends (Next.js, CRA) share
the same three concepts with different config — solved for Vite now (ReMind uses
it), documented as a pattern for others later.

## Distribution

Built and shipped with **GoReleaser**, triggered on a git tag in CI:

- **GitHub Releases** — prebuilt binaries for macOS (Intel + ARM), Linux, and
  Windows. Zero runtime dependencies.
- **Homebrew tap** — `brew install <tap>/lane` for the primary install path.
- **`curl | sh` installer** — one-liner that drops the right binary into
  `/usr/local/bin` for non-Homebrew users.
- `go install` also works for Go developers.

## Deferred (post-v1)

- **HTTPS / TLS** for local hostnames (mkcert or Traefik's local CA). Add if an
  app needs secure cookies or HTTPS-only APIs. v1 is HTTP `:80` only.
- **Non-Vite frontend presets** (Next.js, CRA) — same three concepts as Vite,
  documented as a pattern; bake in presets once needed.
- **General-purpose / third-party project support** — v1 targets the user's own
  projects; generalize after the rough edges are known.
- **Per-slug image-tag isolation** — the Tiltfile hook (`:${LANE_SLUG}`) is in
  place but disabled by default in v1. Until enabled, two worktrees share built
  image tags; live_update isolates active edits, but a simultaneous full rebuild
  in one worktree can update the shared tag the other uses. Enable by setting
  per-slug `image:` tags in compose + the Tiltfile `tag` var.
