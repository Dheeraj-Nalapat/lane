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

## Open / still to design

These are not yet decided and will be brainstormed before implementation:

1. **Slug derivation rules** — exact precedence (explicit flag/env →
   `.berth.toml` base + worktree suffix → directory/branch name), and
   DNS-safe sanitization. Worktree suffixing scheme.
2. **CLI command surface** — likely `up`, `ls`, `down`, `proxy up|down|status`,
   `doctor`; exact flags and output format. Registry derived from Docker labels
   (stateless) vs a state file.
3. **Tilt UI routing** — free host port per instance vs also routing the Tilt
   dashboard through Traefik (`tilt.<slug>.localhost`).
4. **Implementation language / distribution** — Go (single static binary, easy
   `brew`/install) vs Python (fits the user's stack, `uvx`/`pipx`). Affects
   publish story.
5. **Vite-behind-proxy specifics** — exact `server.hmr` / `allowedHosts` config
   and how `/api` proxying resolves to the internal backend in Docker mode.
6. **HTTPS** — deferred; note for future (mkcert/Traefik TLS) if apps need
   secure cookies.
