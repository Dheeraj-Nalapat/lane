# How Lane Works

A complete guide to understanding Lane — what it is, why it exists, and how it does its job.

---

## The Problem Lane Solves (In Plain English)

Imagine a busy port where ships need to dock. If there's only one slot, ships have to wait in line — or worse, crash into each other trying to dock at the same time.

That's exactly what happens when developers run multiple projects (or multiple versions of the same project) on their computer. Each project's development server wants to use the same "port" — like port 8000 for the API, port 5173 for the frontend, or port 80 for the web server. The moment you start a second project, you get an error: **"port already in use."**

**Lane eliminates this problem entirely.** Instead of fighting over shared ports, every project gets its own "lane" and is accessible through a friendly URL like `http://myproject.localhost`.

---

## The One-Sentence Summary

Lane is a CLI tool that lets you run many Docker-based development stacks simultaneously — across different projects and even multiple branches of the same project — with zero port conflicts, by routing traffic through a shared reverse proxy using hostnames instead of ports.

---

## Core Concepts

### 1. The Slug — Your Project's Identity

Every running stack gets a unique **slug** — a short, DNS-safe name that identifies it. This slug determines the URL you'll use to reach it.

**How the slug is determined (in priority order):**

| Priority | Source | Example |
|----------|--------|---------|
| 1st | `--slug` flag | `lane up --slug custom-name` |
| 2nd | `LANE_SLUG` environment variable | `export LANE_SLUG=myapp` |
| 3rd | `.lane.toml` name + worktree suffix | `remind` or `remind-featx` |
| 4th | Directory basename | Whatever your folder is named |

The **worktree suffix** is key: if you're in a git worktree named `featx` for a project named `remind`, your slug becomes `remind-featx`. This means the main checkout and the worktree can run side by side without any conflict.

### 2. The Proxy — The Traffic Controller

Lane runs a single shared [Traefik](https://traefik.io) reverse proxy (called `lane-proxy`) that:

- Owns port 80 on your machine (and optionally port 443 for HTTPS)
- Listens for incoming requests
- Looks at the hostname in the request (e.g., `remind.localhost`)
- Routes the request to the correct container over an internal Docker network

Think of it like a receptionist at a building entrance — they look at who you're trying to visit and direct you to the right office, without you needing to know the room number.

### 3. The Override — Non-Invasive Magic

Lane **never modifies your project files**. Instead, it generates a Compose override file at `~/.lane/overrides/<slug>.override.yml` that:

- Strips all host port bindings (so containers don't fight over ports)
- Resets hardcoded container names (so multiple stacks can coexist)
- Connects routed services to the shared `lane` Docker network
- Adds Traefik routing labels so the proxy knows how to reach each service
- Tags containers with identity labels (used by `lane ls` and `lane view`)

This override is layered on top of your existing `docker-compose.yml` at runtime — your committed files stay byte-for-byte identical.

### 4. The Network — The Internal Highway

All stacks communicate through a shared Docker network called `lane`. The Traefik proxy and all routed services are connected to this network. Since Docker networks are isolated address spaces, there's no port conflict — every container can listen on port 80 internally, and Traefik distinguishes them by hostname labels.

---

## How It Works End-to-End

Here's what happens when you type `lane up`:

```
You type: lane up
    │
    ▼
┌─────────────────────────────────────────────────────┐
│ 1. RESOLVE PROJECT                                  │
│    Find .lane.toml, determine project directory     │
├─────────────────────────────────────────────────────┤
│ 2. PREFLIGHT CHECKS                                │
│    Docker running? Compose ≥ 2.20? Ports free?     │
├─────────────────────────────────────────────────────┤
│ 3. LOAD MANIFEST                                   │
│    Read .lane.toml → name, compose_file, routes    │
├─────────────────────────────────────────────────────┤
│ 4. DERIVE SLUG                                     │
│    Check flag → env → manifest+worktree → dirname  │
├─────────────────────────────────────────────────────┤
│ 5. PARSE COMPOSE                                   │
│    Read services, ports, build contexts            │
├─────────────────────────────────────────────────────┤
│ 6. RESOLVE ROUTES                                  │
│    Merge explicit [[routes]] + auto-routes         │
├─────────────────────────────────────────────────────┤
│ 7. GENERATE OVERRIDE                               │
│    Write ~/.lane/overrides/<slug>.override.yml     │
│    • ports: !reset []                              │
│    • container_name: !reset null                   │
│    • Traefik labels + lane network                 │
├─────────────────────────────────────────────────────┤
│ 8. ENSURE PROXY                                    │
│    Start lane-proxy (Traefik) if not running       │
├─────────────────────────────────────────────────────┤
│ 9. RUN                                             │
│    Tilt: tilt up --host 0.0.0.0 --port <free>     │
│    Compose: docker compose -p <slug> up -d         │
├─────────────────────────────────────────────────────┤
│ 10. READY                                          │
│    Stack is live at http://<slug>.localhost         │
└─────────────────────────────────────────────────────┘
```

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│           YOUR BROWSER                                       │
│                                                             │
│  http://remind.localhost                                     │
│  http://remind-featx.localhost                               │
│  http://otherproject.localhost                               │
└──────────────────────────┬──────────────────────────────────┘
                           │ port 80
                           ▼
┌─────────────────────────────────────────────────────────────┐
│           TRAEFIK (lane-proxy)                               │
│                                                             │
│  • Listens on host :80 (and :443 if TLS enabled)            │
│  • Routes by Host header over the "lane" Docker network     │
│  • Docker provider: reads labels from containers            │
│  • File provider: reads Tilt UI routes from disk            │
└────────┬──────────────────┬──────────────────┬──────────────┘
         │                  │                  │
    lane network       lane network       lane network
         │                  │                  │
         ▼                  ▼                  ▼
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│   remind     │   │ remind-featx │   │ otherproject │
│              │   │              │   │              │
│ ui (no host  │   │ ui (no host  │   │ web (no host │
│  ports)      │   │  ports)      │   │  ports)      │
│ server       │   │ server       │   │ api          │
│ db           │   │ db           │   │ db           │
└──────────────┘   └──────────────┘   └──────────────┘
```

---

## The Two Runners

Lane supports two backends for running your stack:

### Tilt Runner (for live-reload projects)

- Used when a Tiltfile exists in your project (or you set `runner = "tilt"` in `.lane.toml`)
- Provides live-reload: code changes rebuild/restart containers automatically
- Gives you a Tilt dashboard at `http://tilt-<slug>.localhost`
- Requires a small one-time Tiltfile shim (reads `LANE_*` env vars)
- `lane up` runs in the foreground by default (use `-d` for detached)

### Compose Runner (zero-config)

- Used when no Tiltfile is present (or you set `runner = "compose"`)
- Runs plain `docker compose up -d`
- No live-reload, but no extra setup needed — just add `.lane.toml`
- Always runs detached

**Auto-detection:** If a `Tiltfile` exists in your project root, Lane picks the Tilt runner. Otherwise, it uses Compose. You can override this in `.lane.toml`.

---

## Key Features

### Selective Bring-Up (v0.2.0)

Don't need the whole stack? Bring up only what you need:

```bash
lane up api          # only the api service (+ its dependencies)
lane up api ui       # api and ui services
lane up -p debug     # activate the "debug" compose profile
```

Each service gets its own URL: `http://<slug>-<service>.localhost`

### Base-Stack Borrowing (v0.2.0)

Working on a feature branch and only changed the API? Don't spin up a whole new database, frontend, etc.:

```bash
lane up api --base   # fresh api in your worktree, borrow everything else
                     # from the running base stack (main checkout)
```

This connects your worktree's fresh containers to the base stack's shared services — saving memory and startup time.

### Auto-Routing

Every HTTP service is automatically reachable at `<slug>-<service>.localhost`, even without explicit `[[routes]]` in your manifest. You can control this with the `[autoroute]` block:

```toml
[autoroute]
enabled = true        # default
exclude = ["db"]      # don't route the database
```

### HTTPS (Optional)

For apps that need secure cookies or HTTPS-only APIs:

```bash
mkcert -install       # one-time: trust local CA
lane tls enable       # generate wildcard cert, proxy restarts on :443
```

Both `http://` and `https://` then work for all stacks.

### Agent/CI Integration

Lane is designed to work with coding agents (Claude, Cursor, etc.) running in parallel worktrees:

```bash
lane up --wait --json    # wait until serving, output machine-readable JSON
# ...run tests against the URL...
lane down
```

Multiple agents can call `lane up` simultaneously — proxy bring-up is race-safe via a lockfile.

---

## What Lane Touches (and What It Doesn't)

### Lane NEVER modifies:
- Your `docker-compose.yml`
- Your `Tiltfile`
- Your source code
- Any committed file in your repo

### Lane DOES create/manage (all outside your project):
- `~/.lane/overrides/<slug>.override.yml` — generated compose overlay
- `~/.lane/traefik/docker-compose.yml` — the proxy definition
- `~/.lane/traefik/dynamic/<slug>.yml` — Tilt UI route files
- `~/.lane/run/<slug>.pid` and `<slug>.log` — detached process tracking
- `~/.lane/proxy.lock` — race-safe proxy startup lock

### Lane requires IN your project (one-time, committed):
- `.lane.toml` — the manifest (scaffolded by `lane init`)
- Tiltfile shim — only for Tilt projects (reads `LANE_*` env vars)

---

## State Model

Lane is **stateless** by design. It doesn't maintain a database or state file to track what's running. Instead:

- **`lane ls`** queries Docker for containers with `lane.managed` labels
- **`lane view`** reads Docker labels + the Traefik API for live routing info
- **`lane down`** identifies the stack by its slug (from labels) and tears it down

The only "state" files are pidfiles for detached Tilt processes (`~/.lane/run/<slug>.pid`).

---

## The `.lane.toml` Manifest

This is the only file you add to your project. Here's a complete example:

```toml
name = "remind"                            # base slug
compose_file = "infra/docker-compose.yml"  # your compose file path

# Optional: sets LANE_API_TARGET for frontend proxy config
# api_target = "server:8000"

# Optional: force a specific runner
# runner = "tilt"  # or "compose"

# Explicit routes (optional if autoroute is enabled)
[[routes]]
service = "ui"
port = 80
host = "{slug}"    # → remind.localhost (default pattern)

[[routes]]
service = "api"
port = 8000
host = "api.{slug}"  # → api.remind.localhost

# Auto-routing config (optional)
[autoroute]
enabled = true
exclude = ["db", "redis"]
```

You can generate this automatically:
```bash
lane init    # inspects your compose file and scaffolds .lane.toml
```

---

## Commands at a Glance

| Command | What It Does |
|---------|--------------|
| `lane proxy up` | Start the shared Traefik proxy (do this once) |
| `lane up [services...]` | Bring up your stack (or specific services) |
| `lane up -d` | Bring up detached (Tilt runner) |
| `lane up --base` | Fresh services + borrow rest from base stack |
| `lane down` | Tear down the current stack |
| `lane restart` | Restart the current stack |
| `lane ls` | List all running stacks |
| `lane view` | Interactive control panel for all stacks |
| `lane logs` | Tail logs for a stack |
| `lane open` | Open stack URL in browser |
| `lane doctor` | Check system requirements |
| `lane init` | Scaffold `.lane.toml` for your project |
| `lane tls enable` | Enable HTTPS with mkcert |

**Global flags:** `--slug`, `--dry-run`, `-v/--verbose`, `-C/--path <dir>`

---

## System Requirements

| Requirement | Minimum Version | Why |
|-------------|----------------|-----|
| Docker | 28+ | Container runtime |
| Docker Compose | 2.20+ | Uses the `!reset` YAML tag for port stripping |
| Tilt | 0.37+ | Only if using the Tilt runner |
| git | Any recent | Worktree detection |
| mkcert | Any | Only for HTTPS (optional) |

Run `lane doctor` to verify all requirements at once.

---

## Frequently Asked Questions

**Q: Does Lane replace Docker Compose or Tilt?**
No. Lane *orchestrates* them. It generates an override file and sets environment variables, then hands off to Compose or Tilt to do the actual container management.

**Q: Do I need to change my existing docker-compose.yml?**
No. Lane layers its override on top. Your compose file stays untouched.

**Q: What happens if the proxy isn't running?**
`lane up` automatically ensures the proxy is running (locked to prevent races). You can also manage it explicitly with `lane proxy up/down/status`.

**Q: Can two stacks from different projects have the same slug?**
No. Lane detects collisions and errors with a clear message: "slug X already in use by stack at \<path\>". Use `--slug` to disambiguate.

**Q: What about data isolation between stacks?**
Docker Compose prefixes volumes by project name (which is the slug). So each stack gets its own volumes — no data leaks between stacks.

**Q: Does this work on Windows?**
Yes. Lane ships prebuilt binaries for Windows (amd64 + arm64). The `*.localhost` resolution and Docker networking work the same way.

**Q: How do I completely remove Lane from my system?**
1. `lane down` in each project
2. `lane proxy down`
3. `rm -rf ~/.lane`
4. Remove the `lane` binary

---

## Technical Facts at a Glance

- **Language:** Go 1.24, single static binary (~15MB), zero runtime dependencies
- **License:** MIT
- **Proxy:** Traefik v3.1 (Docker image, auto-managed)
- **Network:** Shared external Docker network named `lane`
- **Override format:** YAML with Compose `!reset` tags (requires Compose 2.20+)
- **TUI:** Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) + [Lipgloss](https://github.com/charmbracelet/lipgloss)
- **Config format:** TOML (`.lane.toml`)
- **Platforms:** Linux, macOS, Windows (amd64 + arm64)
- **State:** Stateless — all discovery via Docker labels + Traefik API
- **Release tooling:** GoReleaser + GitHub Actions
- **Current version:** v0.2.0 (June 2026)

---

## The Analogy (TL;DR)

Think of Lane like a **hotel front desk**:

- Each project is a **guest** — they check in and get a room (a unique URL)
- The **Traefik proxy** is the front desk — it knows which guest is in which room and directs visitors accordingly
- The **lane network** is the hotel's internal hallway system — guests don't need to open their doors to the street (no published ports)
- The **slug** is the guest's name tag — stable, unique, and used for all identification
- The **override** is the hotel's way of assigning rooms without the guest needing to renovate their house (non-invasive)

No matter how many guests check in, there's never a collision. Everyone gets their own room and their own address.
