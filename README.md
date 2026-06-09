# berth ⚓

Run many project stacks at once — across different projects **and** multiple git
worktrees of the same project — with **zero host-port conflicts**. Each stack is
reachable in the browser at a friendly `*.localhost` URL via one shared
[Traefik](https://traefik.io) proxy.

> A *berth* is where a ship docks. Every stack gets its own berth instead of
> fighting over a shared port slot.

---

## Why

You develop several projects (and several worktrees) at once, each brought up
with [Tilt](https://tilt.dev) in Docker mode. They all hardcode the same host
ports (`:8000`, `:5173`, `:80`, Tilt's `:10350`…), so the moment a second stack
starts, ports collide.

berth fixes this structurally: stacks publish **no host ports at all**. A shared
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

Run `berth doctor` to check all of these at once.

---

## Install

berth is a single static Go binary — no runtime dependencies.

**From source (current setup):**
```bash
cd /path/to/berth
go build -o ~/.local/bin/berth .   # ~/.local/bin must be on your PATH
berth doctor
```

**Published releases (once a remote/tap is set up):**
```bash
brew install <owner>/berth/berth          # Homebrew
# or
curl -sSL https://github.com/<owner>/berth/releases/latest/download/install.sh | sh
```

> Rebuild after changing berth's code: `go build -o ~/.local/bin/berth .`
> If an open shell can't find it, run `hash -r`.

---

## Quick start

```bash
# 1. Start the shared proxy once (owns :80, creates the `berth` network)
berth proxy up

# 2. In a project that has a .berth.toml + berth-aware Tiltfile:
cd ~/projects/remind
berth up                 # foreground (Tilt logs in your terminal)
#   → http://remind.localhost        (the app)
#   → http://tilt-remind.localhost   (the Tilt UI)

# 3. The same repo in a worktree — at the same time, no conflict:
cd ~/projects/remind-featx
berth up -d              # detached (backgrounds Tilt, prints URLs)
#   → http://remind-featx.localhost

# See what's running
berth ls
berth view               # rich control panel (live routing)

# Stop a stack (leaves your repo byte-for-byte unchanged)
berth down
```

---

## Onboarding a project (one-time)

berth needs two small, **commit-once** additions to a project. It never modifies
them at runtime — it only reads them and sets environment variables. (See
"How it works" for why these can't be auto-injected.)

### 1. `.berth.toml` (project root)

```toml
name = "remind"                       # base slug
compose_file = "infra/docker-compose.yml"   # path to your base compose, relative to repo root

# Optional: for a dev-server frontend that proxies /api to a backend container
# api_target = "server:8000"          # berth sets BERTH_API_TARGET=http://server:8000

[[routes]]
service = "ui"                        # which compose service is the web entrypoint
port = 80                             # its internal container port
# host = "{slug}"                     # default → <slug>.localhost; use "api.{slug}" for extra routes

# Add more [[routes]] blocks for additional web entrypoints if needed.
```

`berth init` will scaffold this for you by inspecting your compose file:
```bash
cd ~/projects/remind
berth init
```

### 2. Tiltfile shim (in the `--docker` branch)

berth's only hook into a Tilt run is environment variables, so your Tiltfile must
read them. In your Docker-mode branch, replace the single `docker_compose(...)`
call with:

```python
if use_docker:
    # --- berth integration (active only under `berth up`) ---
    berth_slug = os.getenv("BERTH_SLUG", "")

    compose_files = ["./infra/docker-compose.yml"]
    berth_override = os.getenv("BERTH_COMPOSE_OVERRIDE", "")
    if berth_override:
        compose_files.append(berth_override)

    # Tilt does NOT honor COMPOSE_PROJECT_NAME — pass the slug as project_name
    # so each stack (and `berth down`) is isolated by the slug, not the dir name.
    if berth_slug:
        docker_compose(compose_files, project_name=berth_slug)
    else:
        docker_compose(compose_files)

    # ... your docker_build() / dc_resource() calls unchanged ...
```

Your Tiltfile must also accept the `--docker` flag (berth always invokes
`tilt up --host 0.0.0.0 --port <free> -- --docker`):
```python
config.define_bool("docker")
cfg = config.parse()
use_docker = cfg.get("docker", False)
```

**This shim is inert without berth.** A plain `tilt up -- --docker` (no `BERTH_*`
env) falls back to the original single `docker_compose(...)` and default project
name — byte-for-byte your old behavior.

> **Commit both files.** New worktrees inherit them automatically. If they're
> uncommitted, a freshly-created worktree won't have them and `berth up` there
> will fail — commit `.berth.toml`, the Tiltfile shim, and any other build fixes.

### Frontend hot-reload (optional)

berth is **service-agnostic** — it routes whatever your manifest names, whether
that's a static build (e.g. nginx on `:80`) or a live dev server. A static UI
needs nothing extra. If you want Vite HMR behind the proxy, run Vite in a
container and gate `vite.config.ts` on the `BERTH` env var:

```ts
const berth = !!process.env.BERTH
export default defineConfig({
  server: {
    host: berth ? '0.0.0.0' : 'localhost',
    allowedHosts: berth ? ['.localhost'] : undefined,
    hmr: berth ? { clientPort: 80 } : undefined,
    proxy: { '/api': { target: process.env.BERTH_API_TARGET || 'http://localhost:8000', changeOrigin: true } },
  },
})
```

---

## Commands

| Command | What it does |
|---|---|
| `berth up [path]` | Bring a stack up: derive slug → generate override → ensure proxy → run `tilt up -- --docker`. **Foreground** by default; `-d/--detach` backgrounds it. |
| `berth down [path]` | Tear down the stack (`docker compose down`) and delete generated files. Repo left untouched. |
| `berth ls` | Quick, scriptable table of running stacks: slug, URL, Tilt port, state, path. |
| `berth view [--watch]` | Rich control panel: each stack → URLs → live Traefik routes (from the Traefik API). `--watch` live-refreshes. |
| `berth proxy up\|down\|status` | Manage the shared Traefik proxy + `berth` network. |
| `berth doctor` | Preflight: Docker, Compose ≥ 2.20, Tilt, `*.localhost` resolution. |
| `berth init` | Scaffold `.berth.toml` by inspecting the project's compose. |
| `berth open` | Open a stack's URL in the browser (`--slug` to pick). |
| `berth logs [path]` | Tail a stack's logs (detached log file, else `docker compose logs -f`). |

**Global flags:** `--slug <x>` (override identity), `--dry-run` (print what would
happen and exit — great for inspecting the generated override), `-v/--verbose`.

---

## How the URL / slug is chosen

Everything (hostname, `COMPOSE_PROJECT_NAME`, Tilt port) derives from one
**slug**, resolved in this order (first match wins):

1. `--slug <x>` flag
2. `BERTH_SLUG` env var
3. `.berth.toml` `name` **+ git worktree suffix** — main checkout = `remind`,
   a linked worktree = `remind-<worktree-name>` (e.g. `remind-featx`)
4. Fallback: the project directory's basename

The worktree suffix comes from the **git worktree name** (stable, tied to the
checkout), not the branch name (which changes). Slugs are sanitized to a safe
DNS label.

---

## How it works

```
┌───────────────────────────────────────────────┐
│ Shared, always-on:  Traefik (berth-proxy)       │
│  • owns host :80                                 │
│  • routes Host(<slug>.localhost) → containers    │
│    over the `berth` Docker network               │
└───────▲───────────────────────▲─────────────────┘
        │ berth network          │ berth network
   ┌────┴─────┐            ┌──────┴──────┐
   │ remind   │            │ remind-featx │   same repo, two worktrees
   │ (no host │            │ (no host     │
   │  ports)  │            │  ports)      │
   └──────────┘            └─────────────┘
```

On `berth up`, berth:
1. Resolves the **slug**.
2. Generates a Compose **override** (`~/.berth/overrides/<slug>.override.yml`)
   that — without touching your committed files —
   - strips host-port publishing (`ports: !reset []`),
   - resets hardcoded `container_name`s (`container_name: !reset null`),
   - joins web services to the `berth` network and adds Traefik routing labels,
   - tags every container with `berth.*` labels (the source of truth for `ls`/`view`/`down`).
3. Writes a Traefik file-provider route for the Tilt UI (`tilt-<slug>.localhost`
   → `host.docker.internal:<tilt-port>`).
4. Sets env (`COMPOSE_PROJECT_NAME`, `BERTH`, `BERTH_SLUG`, `BERTH_COMPOSE_OVERRIDE`,
   optional `BERTH_API_TARGET`) and runs Tilt.

berth holds **no state of its own** — `ls`/`view` read live Docker labels and the
Traefik API. The only runtime file is a pidfile (detached runs only).

### Why the manifest + Tiltfile shim must be committed (not auto-handled)

berth generates the *override* and sets *env vars* every run — those are never
committed. But it cannot inject logic into your Tiltfile, because **Tilt owns
Tiltfile evaluation and berth's only hook is environment variables**. The
Tiltfile must voluntarily read them. Auto-rewriting your Tiltfile each run would
violate berth's core promise of never mutating your committed files. So the
manifest + shim are a one-time, gated opt-in; everything that varies per
run/worktree, berth handles automatically.

---

## On-disk layout (`~/.berth/`)

```
~/.berth/
  overrides/<slug>.override.yml   generated compose overlay (per stack)
  run/<slug>.pid, <slug>.log      detached Tilt process tracking
  traefik/docker-compose.yml      the shared proxy
  traefik/dynamic/<slug>.yml      Tilt-UI routes (file provider)
```

Override `~/.berth` with the `BERTH_HOME` env var. Nothing here is committed to
your projects.

---

## Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `berth: command not found` | Binary not on PATH. `go build -o ~/.local/bin/berth .` (ensure `~/.local/bin` is on PATH), then `hash -r`. |
| `slug "x" already in use by stack at <path>` | Another stack from a different path claimed that slug. Use `--slug` to disambiguate. |
| `berth up` in a new worktree fails to build/parse | The worktree lacks committed `.berth.toml` / Tiltfile shim / Dockerfile fixes. Commit them so worktrees inherit them. |
| Containers named `<dir>-svc-1` instead of `<slug>-svc-1`; `berth down` leaves containers | Tiltfile shim missing `project_name=berth_slug`. Tilt ignores `COMPOSE_PROJECT_NAME`. |
| `unknown flag: --docker` in Tilt | Tiltfile doesn't `config.define_bool("docker")` + `config.parse()`. |
| `tilt-<slug>.localhost` → 502 | Tilt UI not reachable from the proxy. berth passes `--host 0.0.0.0` for this; ensure you're on a current build. |
| `tilt-<slug>.localhost` → 404 right after `up` | Tilt still initializing its UI; retry after a few seconds. |
| `*.localhost` doesn't resolve | `berth doctor` will flag it; configure your resolver to map `.localhost` → 127.0.0.1. |
| Two stacks of one project share data | Compose prefixes volumes by project name (the slug), so they're isolated — verify the `project_name` shim is present. |

---

## Limitations (v1)

- **Per-slug image-tag isolation is disabled by default.** Two worktrees share
  built image tags; `live_update` isolates active edits, but a simultaneous full
  rebuild in one worktree can update the shared tag the other uses. (Hook present
  in the Tiltfile `tag` var — enable by setting per-slug `image:` tags.)
- **HTTP only** (no TLS for `*.localhost`).
- **Docker mode only** — built around Tilt's `--docker` path.
- **Your own projects first** — not yet hardened for arbitrary third-party repos.

See `docs/2026-06-08-berth-design.md` for the full design rationale and
`docs/DEVELOPMENT.md` for contributing.
