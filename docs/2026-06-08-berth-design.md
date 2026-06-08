# berth — Design Spec (v0.1, WIP)

**Date:** 2026-06-08
**Status:** Brainstorm in progress. This captures decisions made so far; the
"Open / still to design" section lists what remains.

> **Name:** *berth* — a place a ship docks. Every stack gets its own berth
> instead of fighting over a shared port slot. CLI verbs: `berth up`,
> `berth ls`, `berth down`.

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
- A **reusable convention** plus a publishable CLI tool (`berth`), not a
  one-off hack.
- **Non-invasive**: using berth must never change behavior in
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
| Packaging        | **`berth` CLI** as the cockpit over a Traefik + slug mechanism |
| v1 audience      | My own projects first, publishable-quality, not yet general-purpose |
| Override style   | berth **generates** the proxy/network/port wiring; committed files stay pristine |
| Language         | **Go** — single static binary, zero runtime deps; users get a dependency-free artifact regardless of their own stack (TS/Java/etc.) |

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Shared, always-on:  Traefik  (network: berth)            │
│  • owns host :80                                           │
│  • watches the Docker socket, auto-discovers labeled       │
│    containers                                              │
│  • routes Host(<slug>.localhost) → that container's port   │
└───────────────▲─────────────────────▲─────────────────────┘
                │ berth network        │ berth network
     ┌──────────┴─────────┐  ┌─────────┴──────────┐
     │ Stack: remind      │  │ Stack: remind-featx │   ← same repo,
     │ COMPOSE_PROJECT=…  │  │ COMPOSE_PROJECT=…   │     two worktrees
     │ ui, server, agent  │  │ ui, server, agent   │
     │ (no host ports)    │  │ (no host ports)     │
     └────────────────────┘  └─────────────────────┘
                ▲                       ▲
                └────────── berth CLI ───┘
        (derives slug, sets env, runs tilt, lists / tears down)
```

Three cleanly separated pieces:

1. **Traefik (shared infra).** Started once, owns host `:80` and the external
   `berth` Docker network. Watches the Docker socket and routes by hostname.
   Knows nothing about any project.

2. **Each stack (a normal compose project).** Must, *via generated overlay*:
   join the `berth` network, publish **no host ports**, and carry Traefik
   labels naming its hostname. Inter-service calls stay on the stack's own
   private compose network (`agent-server:8100`), unchanged from today.

3. **berth CLI (the cockpit).** Derives the per-stack `SLUG`, exports
   `COMPOSE_PROJECT_NAME` + a free Tilt UI port + a per-slug image-tag
   namespace, runs `tilt up -- --docker`, and reads Docker labels back for
   `ls`/`down`. **Holds no state of its own** — truth lives in Docker, so
   nothing drifts.

### Why no host ports = no conflicts

`ports: ["8000:8000"]` binds container port 8000 to **host** port 8000 — a
finite, machine-wide resource only one process can own. That is the root cause
of the collisions. If stacks don't publish host ports, the container still
listens internally (reachable inside Docker networks) but nothing touches the
host. Traefik reaches containers **over the `berth` network**, not via host
ports, and fans them out by hostname. Only Traefik owns a host port (`:80`).

## The per-project contract

Split into what's **committed** vs **generated** (the split is what keeps churn
near zero and guarantees non-invasiveness):

**Committed to the repo (small, one-time):**
- A `.berth.toml` manifest: project base-name, and which service(s) are web
  entrypoints + their internal ports.
- For any **Vite** UI: `server.host = '0.0.0.0'` and `server.allowedHosts`
  accepting the custom hostname, plus an HMR clientPort setting for behind-proxy
  websockets. **Env-gated** so they're inert outside berth.

**Generated by berth at `up` time (never committed):**
- A compose override (`*.berth.override.yml`) that:
  - attaches services to the `berth` network,
  - adds Traefik router/service labels with `Host(<slug>.localhost)`,
  - **strips host-port publishing** from the base compose using the
    `ports: !reset []` YAML tag (Docker Compose ≥ 2.20) — so published ports in
    the committed file are neutralized **without editing it**. A service that
    genuinely needs a direct host port can instead be re-published as an
    ephemeral loopback port (`127.0.0.1::<port>`) and reported.
- Environment: `COMPOSE_PROJECT_NAME=<slug>`, a free Tilt UI port, a per-slug
  image-tag namespace.

## Isolation guarantees

- **Non-invasive / production-safe.** berth never mutates committed files.
  All wiring lives in a generated override + environment that exist only during
  a `berth up`. Committed compose, Dockerfiles, and app code are untouched.
  It rides Tilt's existing dev-only `--docker` path. Vite tweaks are env-gated
  no-ops in normal `npm run dev`. Production deploys never run this. Worst case,
  `berth down` leaves the project byte-for-byte unchanged.
- **Worktree independence.** Each worktree runs its **own** `tilt up` process,
  own Tilt UI port, and own compose project. Tilt only watches its own
  worktree's files, so editing worktree A triggers rebuild/restart **only in
  A** — B never sees it. To make isolation total, berth **namespaces built
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
2. `BERTH_SLUG` env var — same, for scripting.
3. `.berth.toml` `name` (base) **+ auto worktree suffix** — the normal path.
4. Fallback: sanitized directory basename (so `berth up` still works in an
   un-onboarded project; less pretty).

### Normal path (worktree-aware)

The committed `.berth.toml` carries the **base name** (`name = "remind"`).
Because it's committed, every worktree inherits the same base. berth then
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
- Truth lives in Docker labels, so `berth up` checks whether the resolved slug
  is **already claimed by a stack from a different path** and refuses with a
  clear message rather than silently colliding.

## Open / still to design

These are not yet decided and will be brainstormed before implementation:

1. ~~Slug derivation rules~~ — **decided** (see "Slug derivation" above).
2. **CLI command surface** — likely `up`, `ls`, `down`, `proxy up|down|status`,
   `doctor`; exact flags and output format. Registry derived from Docker labels
   (stateless) vs a state file.
3. **Tilt UI routing** — free host port per instance vs also routing the Tilt
   dashboard through Traefik (`tilt.<slug>.localhost`).
4. **Distribution mechanics** — language is decided (**Go**, see Decisions).
   Remaining: release channels (GitHub Releases prebuilt binaries, Homebrew tap,
   `go install`, `curl | sh` installer) and the build/CI pipeline.
5. **Vite-behind-proxy specifics** — exact `server.hmr` / `allowedHosts` config
   and how `/api` proxying resolves to the internal backend in Docker mode.
6. **HTTPS** — deferred; note for future (mkcert/Traefik TLS) if apps need
   secure cookies.
