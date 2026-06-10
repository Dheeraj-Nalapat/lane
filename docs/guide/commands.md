---
description: Full lane command reference — up, down, restart, ls, view, proxy, tls, init, open, logs, and doctor, with their flags, JSON output, and exit codes.
---

# Command reference

| Command | What it does |
|---|---|
| `lane up [path]` | Bring a stack up. Tilt runner: foreground (`-d` to detach). Compose runner: detached; `--build` rebuilds. `--json` prints `{slug,urls[]}`; `--wait` blocks until serving (`--wait-timeout`, default 90s). |
| `lane down [path]` | Tear down the stack; `--volumes` also removes named volumes. |
| `lane restart [path]` | Recreate (down then up). |
| `lane ls` | List running stacks; `--json` for machine output. |
| `lane view` | Interactive control panel (TTY); `--plain` / piped prints a static snapshot. |
| `lane proxy up\|down\|status` | Manage the shared Traefik proxy. |
| `lane tls enable\|disable\|status` | Optional HTTPS via mkcert. |
| `lane init` | Scaffold `.lane.toml` from your compose. |
| `lane open` / `lane logs` | Open a stack's URL / tail its logs. |
| `lane doctor` | Preflight checks (Docker, Compose ≥ 2.20, `*.localhost`). |

Global flags: `--slug` (override identity), `--dry-run`, `-v/--verbose`.
Exit codes: `0` success (incl. already-running); `1` error.
