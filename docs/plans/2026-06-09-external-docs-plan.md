# External Docs & Recipes (E) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `docs/guide/` set — a getting-started tutorial and three onboarding recipes (plain compose, Tilt, frontend HMR) — accurate against the shipped CLI and ready for the future website.

**Architecture:** Standalone markdown under `docs/guide/`, linked from the README. No code changes. "Tests" are accuracy checks: shown commands/flags exist in the built `lane`, and internal links resolve.

**Tech Stack:** Markdown. Verified facts from the spec (incl. the live Next.js HMR check).

Spec: `docs/2026-06-09-external-docs-design.md`.

---

## File Structure

```
docs/guide/README.md                  index / reading order
docs/guide/getting-started.md         zero → two stacks tutorial
docs/guide/recipes/compose.md         plain docker-compose (no Tiltfile)
docs/guide/recipes/tilt.md            Tilt project (gated shim) → links onboarding-remind.md
docs/guide/recipes/frontend-hmr.md    Vite + Next.js dev servers behind the proxy
README.md                             + "Guides" pointer
```

Each doc is self-contained; commit per file.

---

### Task 1: `docs/guide/getting-started.md`

**Files:** Create `docs/guide/getting-started.md`

- [ ] **Step 1: Write the tutorial**

```markdown
# Getting started with lane

lane runs your project's stack behind a shared proxy so it's reachable at a
friendly `*.localhost` URL — and so **every git worktree gets its own isolated
stack with no port conflicts**. This is the first 10 minutes.

## Prerequisites

- Docker (≥ 28) with Compose ≥ 2.20
- git
- Tilt — only if your project uses it (plain docker-compose projects don't need it)

Check everything at once:

```bash
lane doctor
```

## 1. Onboard your project

From a project that has a `docker-compose.yml` (or a Tiltfile + compose):

```bash
lane init        # writes a .lane.toml (inspects your compose to guess the web service)
```

Review the generated `.lane.toml`:

```toml
name = "myapp"
compose_file = "docker-compose.yml"

[[routes]]
service = "web"
port = 3000
```

## 2. Bring it up

```bash
lane up
```

lane starts the shared proxy (first time), generates a non-invasive compose
overlay, and brings your stack up. Open the printed URL:

```
http://myapp.localhost
```

See what's running:

```bash
lane ls        # quick table
lane view      # live, interactive control panel (q to quit)
```

## 3. A second worktree — at the same time

The payoff: run the *same* project twice without port clashes.

```bash
git worktree add ../myapp-featx -b featx
cd ../myapp-featx
lane up
```

Now both are reachable simultaneously:

```
http://myapp.localhost          # main checkout
http://myapp-featx.localhost    # the worktree
```

The slug (and URL) is derived from the git worktree, so they never collide.

## 4. Tear down

```bash
lane down            # this stack (keeps volumes)
lane down --volumes  # also wipe its named volumes
```

## Next steps

- Your project type: [plain compose](recipes/compose.md) ·
  [Tilt](recipes/tilt.md) · [frontend HMR](recipes/frontend-hmr.md)
- HTTPS, and driving lane from coding agents (parallel testing): see the
  [README](../../README.md).
```

- [ ] **Step 2: Commit**

```bash
git add docs/guide/getting-started.md
git commit -m "docs(guide): getting-started tutorial"
```

---

### Task 2: `docs/guide/recipes/compose.md`

**Files:** Create `docs/guide/recipes/compose.md`

- [ ] **Step 1: Write the recipe**

```markdown
# Recipe: a plain docker-compose project

If your project has only a `docker-compose.yml` (no Tiltfile), lane drives
Compose directly — **no Tiltfile and no shim required.** This is the most common
case.

## 1. Manifest

```bash
lane init
```

→ `.lane.toml`:

```toml
name = "myapp"
compose_file = "docker-compose.yml"

[[routes]]
service = "web"   # the service that serves your app
port = 3000       # its container port
```

(Add more `[[routes]]` blocks for extra web entrypoints; `host = "api.{slug}"`
gives `api.<slug>.localhost`.)

## 2. Up / down

```bash
lane up          # compose runner, detached; prints the URL
lane up --build  # force a rebuild of built services
lane down
```

`lane up` is detached for compose projects (`-d` is a no-op). Re-running `lane up`
on a running stack no-ops; use `lane restart` to recreate.

## Notes

- **Built-image isolation is automatic.** For services with a `build:` section,
  lane lets Compose name the image per project (slug), so two worktrees never
  clobber each other's image.
- **No host ports are published** — lane strips them and routes via the proxy,
  which is why parallel stacks don't conflict.
- For machine use (agents/CI): `lane up --wait --json` waits until the route
  serves and prints `{slug, urls[]}`.
```

- [ ] **Step 2: Commit**

```bash
git add docs/guide/recipes/compose.md
git commit -m "docs(guide): plain docker-compose recipe"
```

---

### Task 3: `docs/guide/recipes/tilt.md`

**Files:** Create `docs/guide/recipes/tilt.md`

- [ ] **Step 1: Write the recipe**

```markdown
# Recipe: a Tilt project

If your project uses [Tilt](https://tilt.dev), lane drives it (keeping
live-reload + the Tilt dashboard). lane auto-detects the Tiltfile and uses the
Tilt runner. You add a small, **gated** shim to your Tiltfile's docker branch.

## 1. Manifest

```toml
name = "myapp"
compose_file = "infra/docker-compose.yml"

[[routes]]
service = "ui"
port = 80
```

## 2. Tiltfile shim (in the `--docker` branch)

lane's only hook is environment variables, so the Tiltfile must read them. In the
Docker-mode branch:

```python
config.define_bool("docker")
cfg = config.parse()
use_docker = cfg.get("docker", False)

if use_docker:
    lane_slug = os.getenv("LANE_SLUG", "")
    tag = (":" + lane_slug) if lane_slug else ""   # per-slug image isolation

    compose_files = ["./infra/docker-compose.yml"]
    override = os.getenv("LANE_COMPOSE_OVERRIDE", "")
    if override:
        compose_files.append(override)

    # Tilt does NOT honor COMPOSE_PROJECT_NAME — pass the slug as project_name.
    if lane_slug:
        docker_compose(compose_files, project_name=lane_slug)
    else:
        docker_compose(compose_files)

    docker_build("myapp/server" + tag, context=".", dockerfile="infra/Dockerfile", live_update=[...])
    # dc_resource(...) as before
```

This is inert without lane: a plain `tilt up -- --docker` (no `LANE_*` env) falls
back to the original single `docker_compose(...)`.

## 3. Up / down

```bash
lane up           # foreground (Tilt logs); -d to detach
lane down
```

The Tilt UI is routed at `http://tilt-<slug>.localhost`.

## Worked example

See [`docs/onboarding-remind.md`](../../onboarding-remind.md) — the ReMind
project onboarded end-to-end (two worktrees running at once).
```

- [ ] **Step 2: Commit**

```bash
git add docs/guide/recipes/tilt.md
git commit -m "docs(guide): Tilt project recipe"
```

---

### Task 4: `docs/guide/recipes/frontend-hmr.md`

**Files:** Create `docs/guide/recipes/frontend-hmr.md`

- [ ] **Step 1: Write the recipe**

```markdown
# Recipe: a dev server with hot-reload (Vite / Next.js)

To run a frontend **dev server** (with HMR) behind the lane proxy, two things
must hold:

1. The dev server binds `0.0.0.0` (so the container is reachable).
2. If the dev server checks the `Host` header, it must allow `*.localhost`.

HMR rides the same `:80` through Traefik (WebSockets pass through).

## Vite

Vite blocks unknown `Host` headers and needs its HMR client pointed at the proxy
port. Gate it on the `LANE` env var so normal `npm run dev` is unchanged:

```ts
// vite.config.ts
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

lane sets `LANE=1` and (if `api_target` is in `.lane.toml`)
`LANE_API_TARGET=http://<service>:<port>` when it runs.

## Next.js

Next's dev server works behind lane **with no special config** — just bind the
host. Run it as your service's command and route the port:

```yaml
# docker-compose.yml (dev)
services:
  web:
    image: node:20-alpine
    working_dir: /app
    command: sh -c "npm install && npm run dev"   # package.json: "next dev -H 0.0.0.0 -p 3000"
    volumes: [".:/app"]
```

```toml
# .lane.toml
[[routes]]
service = "web"
port = 3000
```

Verified with Next 16 (Turbopack): the app served over `http://<slug>.localhost`
and the HMR websocket (`/_next/webpack-hmr`) upgraded cleanly through the proxy —
no extra config. If an older/other Next version logs a cross-origin dev warning,
add to `next.config.js`:

```js
module.exports = { allowedDevOrigins: ['*.localhost'] }
```

## Other frameworks

Apply the two rules above: bind `0.0.0.0`, and allow the `*.localhost` host if the
server host-checks. That covers most dev servers.
```

- [ ] **Step 2: Commit**

```bash
git add docs/guide/recipes/frontend-hmr.md
git commit -m "docs(guide): frontend HMR recipe (Vite + Next.js)"
```

---

### Task 5: `docs/guide/README.md` (index) + README pointer

**Files:** Create `docs/guide/README.md`; modify `README.md`

- [ ] **Step 1: Write the index**

`docs/guide/README.md`:
```markdown
# lane guides

1. **[Getting started](getting-started.md)** — install → up → two stacks at once.
2. Pick the recipe for your project:
   - [Plain docker-compose](recipes/compose.md) (no Tiltfile)
   - [Tilt project](recipes/tilt.md)
   - [Frontend dev server / HMR](recipes/frontend-hmr.md) (Vite, Next.js)
3. Then see the [README](../../README.md) for HTTPS and using lane with coding
   agents (parallel testing).
```

- [ ] **Step 2: Add a "Guides" pointer to the README**

In `README.md`, immediately after the badges/intro (before "## Why"), add:
```markdown
> **New here?** Start with the [guides](docs/guide/README.md): a
> [getting-started tutorial](docs/guide/getting-started.md) and per-project
> recipes (compose, Tilt, frontend HMR).
```

- [ ] **Step 3: Commit**

```bash
git add docs/guide/README.md README.md
git commit -m "docs(guide): index + README pointer"
```

---

### Task 6: Accuracy verification

**No new files. Verification gate.**

- [ ] **Step 1: Every command/flag shown actually exists**

Run (build first if needed: `go build -o ./bin/lane .`):
```bash
L=./bin/lane
$L doctor --help >/dev/null
$L init --help >/dev/null
$L up --help    | grep -qE -- '--build' && $L up --help | grep -qE -- '--wait' && $L up --help | grep -qE -- '--json'
$L down --help  | grep -qE -- '--volumes'
$L ls --help    | grep -qE -- '--json'
$L view --help  | grep -qE -- '--plain'
$L restart --help >/dev/null
$L proxy --help >/dev/null && $L tls --help >/dev/null
echo "all referenced commands/flags exist"
```
Expected: prints "all referenced commands/flags exist" with no grep failure.

- [ ] **Step 2: Internal links resolve**

```bash
# every (relative) markdown link target under docs/guide exists
cd docs/guide
miss=0
for f in $(find . -name '*.md'); do
  d=$(dirname "$f")
  grep -oE '\]\(([^)]+\.md)[^)]*\)' "$f" | sed -E 's/.*\(([^)# ]+).*/\1/' | while read -r link; do
    case "$link" in http*) continue;; esac
    [ -e "$d/$link" ] || { echo "BROKEN: $f → $link"; }
  done
done
echo "link check done"
```
Expected: no `BROKEN:` lines. (Resolve any by fixing the path.)

- [ ] **Step 3: Commit any fixes**

```bash
git commit -am "docs(guide): fix command/link drift" || echo "nothing to fix"
```

---

## Final verification

- [ ] `docs/guide/` has: `README.md`, `getting-started.md`, and the three recipes.
- [ ] Every command/flag shown exists in the built `lane` (Task 6 Step 1 passes).
- [ ] All internal links resolve (Task 6 Step 2: no `BROKEN:`).
- [ ] README has the "Guides" pointer.
- [ ] No code changed; `git status` clean after commits.
