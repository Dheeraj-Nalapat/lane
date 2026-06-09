# lane 🛣️

[![CI](https://github.com/Dheeraj-Nalapat/lane/actions/workflows/ci.yml/badge.svg)](https://github.com/Dheeraj-Nalapat/lane/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Dheeraj-Nalapat/lane)](https://github.com/Dheeraj-Nalapat/lane/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Run many project stacks at once — across different projects **and** multiple git
worktrees of the same project — with **zero host-port conflicts**. Each stack is
reachable in the browser at a friendly `*.localhost` URL via one shared
[Traefik](https://traefik.io) proxy.

> Like lanes on a road, every stack runs in its own lane instead of colliding
> on a shared port.

> **New here?** Start with the [guides](docs/guide/README.md): a
> [getting-started tutorial](docs/guide/getting-started.md) and per-project
> recipes (compose, Tilt, frontend HMR).
>
> The guides also build into a website (MkDocs Material). Locally:
> `pip install -r requirements-docs.txt && mkdocs serve`. It auto-deploys to
> GitHub Pages on push to `main` once Pages is enabled (Settings → Pages →
> Source: GitHub Actions).

---

## Why

You develop several projects (and several worktrees) at once, each brought up
with [Tilt](https://tilt.dev) in Docker mode. They all hardcode the same host
ports (`:8000`, `:5173`, `:80`, Tilt's `:10350`…), so the moment a second stack
starts, ports collide.

lane fixes this structurally: stacks publish **no host ports at all**. A shared
Traefik proxy reaches each container over a Docker network and routes by
hostname, so any number of stacks coexist:

```
http://remind.localhost          → your main checkout
http://remind-featx.localhost    → the same repo in a worktree
http://otherproject.localhost    → a different project
```

---

## Requirements

- **Docker** ≥ 28 and **Docker Compose** ≥ 2.20 (the override uses the `!reset` tag)
- **Tilt** ≥ 0.37
- **git**
- `*.localhost` must resolve to loopback (default on Linux/macOS and modern browsers)

Run `lane doctor` to check all of these at once.

---

## Install

lane is a single static Go binary — no runtime dependencies.

**From source (current setup):**
```bash
cd /path/to/lane
go build -o ~/.local/bin/lane .   # ~/.local/bin must be on your PATH
lane doctor
```

**Published releases (once a remote/tap is set up):**
```bash
brew install <owner>/lane/lane          # Homebrew
# or
curl -sSL https://github.com/<owner>/lane/releases/latest/download/install.sh | sh
```

> Rebuild after changing lane's code: `go build -o ~/.local/bin/lane .`
> If an open shell can't find it, run `hash -r`.

---

## Quick start

```bash
# 1. Start the shared proxy once (owns :80, creates the `lane` network)
lane proxy up

# 2. In a project that has a .lane.toml + lane-aware Tiltfile:
cd ~/projects/remind
lane up                 # foreground (Tilt logs in your terminal)
#   → http://remind.localhost        (the app)
#   → http://tilt-remind.localhost   (the Tilt UI)

# 3. The same repo in a worktree — at the same time, no conflict:
cd ~/projects/remind-featx
lane up -d              # detached (backgrounds Tilt, prints URLs)
#   → http://remind-featx.localhost

# See what's running
lane ls
lane view               # rich control panel (live routing)

# Stop a stack (leaves your repo byte-for-byte unchanged)
lane down
```

---

## Onboarding a project (one-time)

A **plain docker-compose project needs only one file** — `.lane.toml` below.
lane auto-detects the absence of a Tiltfile and drives `docker compose` directly
(no shim). **Tilt projects** add a small, commit-once Tiltfile shim (step 2) to
keep live-reload + the Tilt dashboard. lane never modifies these at runtime — it
only reads them and sets environment variables.

### 1. `.lane.toml` (project root)

```toml
name = "remind"                       # base slug
compose_file = "infra/docker-compose.yml"   # path to your base compose, relative to repo root

# Optional: for a dev-server frontend that proxies /api to a backend container
# api_target = "server:8000"          # lane sets LANE_API_TARGET=http://server:8000

[[routes]]
service = "ui"                        # which compose service is the web entrypoint
port = 80                             # its internal container port
# host = "{slug}"                     # default → <slug>.localhost; use "api.{slug}" for extra routes

# Add more [[routes]] blocks for additional web entrypoints if needed.
```

`lane init` will scaffold this for you by inspecting your compose file:
```bash
cd ~/projects/remind
lane init
```

### 2. Tiltfile shim — **Tilt projects only**

**Skip this entirely if your project has no Tiltfile** — lane runs `docker
compose` directly (project name set via `-p <slug>`), so a plain compose project
needs only the `.lane.toml` above. The steps below apply only when you use Tilt
for live-reload + the Tilt dashboard.

lane's only hook into a Tilt run is environment variables, so your Tiltfile must
read them. In your Docker-mode branch, replace the single `docker_compose(...)`
call with:

```python
if use_docker:
    # --- lane integration (active only under `lane up`) ---
    lane_slug = os.getenv("LANE_SLUG", "")

    compose_files = ["./infra/docker-compose.yml"]
    lane_override = os.getenv("LANE_COMPOSE_OVERRIDE", "")
    if lane_override:
        compose_files.append(lane_override)

    # Tilt does NOT honor COMPOSE_PROJECT_NAME — pass the slug as project_name
    # so each stack (and `lane down`) is isolated by the slug, not the dir name.
    if lane_slug:
        docker_compose(compose_files, project_name=lane_slug)
    else:
        docker_compose(compose_files)

    # ... your docker_build() / dc_resource() calls unchanged ...
```

Your Tiltfile must also accept the `--docker` flag (lane always invokes
`tilt up --host 0.0.0.0 --port <free> -- --docker`):
```python
config.define_bool("docker")
cfg = config.parse()
use_docker = cfg.get("docker", False)
```

**This shim is inert without lane.** A plain `tilt up -- --docker` (no `LANE_*`
env) falls back to the original single `docker_compose(...)` and default project
name — byte-for-byte your old behavior.

> **Commit both files.** New worktrees inherit them automatically. If they're
> uncommitted, a freshly-created worktree won't have them and `lane up` there
> will fail — commit `.lane.toml`, the Tiltfile shim, and any other build fixes.

### Frontend hot-reload (optional)

lane is **service-agnostic** — it routes whatever your manifest names, whether
that's a static build (e.g. nginx on `:80`) or a live dev server. A static UI
needs nothing extra. If you want Vite HMR behind the proxy, run Vite in a
container and gate `vite.config.ts` on the `LANE` env var:

```ts
const lane = !!process.env.LANE
export default defineConfig({
  server: {
    host: lane ? '0.0.0.0' : 'localhost',
    allowedHosts: lane ? ['.localhost'] : undefined,
    hmr: lane ? { clientPort: 80 } : undefined,
    proxy: { '/api': { target: process.env.LANE_API_TARGET || 'http://localhost:8000', changeOrigin: true } },
  },
})
```

---

## Commands

| Command | What it does |
|---|---|
| `lane up [path]` | Bring a stack up: derive slug → generate override → ensure proxy → run the selected runner. **Tilt runner:** foreground by default, `-d/--detach` to background. **Compose runner:** always detached; `-d` is a no-op, `--build` forces an image rebuild. |
| `lane down [path]` | Tear down the stack (`docker compose down`) and delete generated files. Repo left untouched. |
| `lane ls` | Quick, scriptable table of running stacks: slug, URL, Tilt port, state, path. |
| `lane view` | Live, interactive control panel (master/detail): select a stack and `o`pen / `l`ogs / `r`estart / `x` down it; auto-refreshing. Piped/CI (non-TTY) prints a static snapshot; `--plain` forces it. |
| `lane proxy up\|down\|status` | Manage the shared Traefik proxy + `lane` network. |
| `lane doctor` | Preflight: Docker, Compose ≥ 2.20, Tilt, `*.localhost` resolution. |
| `lane init` | Scaffold `.lane.toml` by inspecting the project's compose. |
| `lane open` | Open a stack's URL in the browser (`--slug` to pick). |
| `lane logs [path]` | Tail a stack's logs (detached log file, else `docker compose logs -f`). |

**Global flags:** `--slug <x>` (override identity), `--dry-run` (print what would
happen and exit — great for inspecting the generated override), `-v/--verbose`.

---

## HTTPS (optional)

lane serves plain HTTP by default. To get trusted `https://<slug>.localhost`
(for secure cookies / HTTPS-only APIs), install [mkcert](https://github.com/FiloSottile/mkcert),
set up its CA once, then enable TLS:

```bash
mkcert -install        # one-time; adds a local CA to your trust store
lane tls enable        # generates a wildcard cert, restarts the proxy on :443
lane up                # (re-up running stacks to add their HTTPS route)
```

Both `http://` and `https://` then serve every stack. `lane tls status` shows
the current state; `lane tls disable` returns to HTTP-only. mkcert is **not**
required for normal (HTTP) use — only to enable HTTPS.

---

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
stacks. N parallel `lane up`s are race-safe (the shared proxy bring-up is locked).

**Skill files:** a Claude Code skill (`agent/claude`, installable as a plugin —
`/plugin marketplace add Dheeraj-Nalapat/lane`) and a Cursor rule
(`agent/cursor/lane.mdc`).

---

## How the URL / slug is chosen

Everything (hostname, `COMPOSE_PROJECT_NAME`, Tilt port) derives from one
**slug**, resolved in this order (first match wins):

1. `--slug <x>` flag
2. `LANE_SLUG` env var
3. `.lane.toml` `name` **+ git worktree suffix** — main checkout = `remind`,
   a linked worktree = `remind-<worktree-name>` (e.g. `remind-featx`)
4. Fallback: the project directory's basename

The worktree suffix comes from the **git worktree name** (stable, tied to the
checkout), not the branch name (which changes). Slugs are sanitized to a safe
DNS label.

---

## How it works

```
┌───────────────────────────────────────────────┐
│ Shared, always-on:  Traefik (lane-proxy)       │
│  • owns host :80                                 │
│  • routes Host(<slug>.localhost) → containers    │
│    over the `lane` Docker network               │
└───────▲───────────────────────▲─────────────────┘
        │ lane network          │ lane network
   ┌────┴─────┐            ┌──────┴──────┐
   │ remind   │            │ remind-featx │   same repo, two worktrees
   │ (no host │            │ (no host     │
   │  ports)  │            │  ports)      │
   └──────────┘            └─────────────┘
```

On `lane up`, lane:
1. Resolves the **slug**.
2. Generates a Compose **override** (`~/.lane/overrides/<slug>.override.yml`)
   that — without touching your committed files —
   - strips host-port publishing (`ports: !reset []`),
   - resets hardcoded `container_name`s (`container_name: !reset null`),
   - joins web services to the `lane` network and adds Traefik routing labels,
   - tags every container with `lane.*` labels (the source of truth for `ls`/`view`/`down`).
3. Writes a Traefik file-provider route for the Tilt UI (`tilt-<slug>.localhost`
   → `host.docker.internal:<tilt-port>`).
4. Sets env (`COMPOSE_PROJECT_NAME`, `LANE`, `LANE_SLUG`, `LANE_COMPOSE_OVERRIDE`,
   optional `LANE_API_TARGET`) and runs Tilt.

lane holds **no state of its own** — `ls`/`view` read live Docker labels and the
Traefik API. The only runtime file is a pidfile (detached runs only).

### Why the manifest + Tiltfile shim must be committed (not auto-handled)

lane generates the *override* and sets *env vars* every run — those are never
committed. But it cannot inject logic into your Tiltfile, because **Tilt owns
Tiltfile evaluation and lane's only hook is environment variables**. The
Tiltfile must voluntarily read them. Auto-rewriting your Tiltfile each run would
violate lane's core promise of never mutating your committed files. So the
manifest + shim are a one-time, gated opt-in; everything that varies per
run/worktree, lane handles automatically.

---

## On-disk layout (`~/.lane/`)

```
~/.lane/
  overrides/<slug>.override.yml   generated compose overlay (per stack)
  run/<slug>.pid, <slug>.log      detached Tilt process tracking
  traefik/docker-compose.yml      the shared proxy
  traefik/dynamic/<slug>.yml      Tilt-UI routes (file provider)
```

Override `~/.lane` with the `LANE_HOME` env var. Nothing here is committed to
your projects.

---

## Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `lane: command not found` | Binary not on PATH. `go build -o ~/.local/bin/lane .` (ensure `~/.local/bin` is on PATH), then `hash -r`. |
| `slug "x" already in use by stack at <path>` | Another stack from a different path claimed that slug. Use `--slug` to disambiguate. |
| `lane up` in a new worktree fails to build/parse | The worktree lacks committed `.lane.toml` / Tiltfile shim / Dockerfile fixes. Commit them so worktrees inherit them. |
| Containers named `<dir>-svc-1` instead of `<slug>-svc-1`; `lane down` leaves containers | Tiltfile shim missing `project_name=lane_slug`. Tilt ignores `COMPOSE_PROJECT_NAME`. |
| `unknown flag: --docker` in Tilt | Tiltfile doesn't `config.define_bool("docker")` + `config.parse()`. |
| `tilt-<slug>.localhost` → 502 | Tilt UI not reachable from the proxy. lane passes `--host 0.0.0.0` for this; ensure you're on a current build. |
| `tilt-<slug>.localhost` → 404 right after `up` | Tilt still initializing its UI; retry after a few seconds. |
| `*.localhost` doesn't resolve | `lane doctor` will flag it; configure your resolver to map `.localhost` → 127.0.0.1. |
| Two stacks of one project share data | Compose prefixes volumes by project name (the slug), so they're isolated — verify the `project_name` shim is present. |

---

## Limitations (v1)

- **Per-slug image isolation:** the **compose runner** does this automatically
  (lane resets the `image:` of `build:` services so Compose names them per
  slug). **Tilt projects** opt in with the slug-tag pattern (`tag = (":" +
  lane_slug) ...` on `docker_build` refs — see `docs/onboarding-remind.md`).
- **HTTP by default; HTTPS is opt-in** via `lane tls` (mkcert). Nested hosts
  like `api.<slug>.localhost` aren't covered by the wildcard cert.
- **Docker mode only** — built around Tilt's `--docker` path / plain compose.
- **Your own projects first** — not yet hardened for arbitrary third-party repos.

See `docs/2026-06-08-lane-design.md` for the full design rationale and
`docs/DEVELOPMENT.md` for contributing.
