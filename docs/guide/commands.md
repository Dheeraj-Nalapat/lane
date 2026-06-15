---
description: Full lane command reference — up, down, restart, ls, view, proxy, tls, init, open, logs, and doctor, with their flags, JSON output, and exit codes.
---

# Command reference

| Command | What it does |
|---|---|
| `lane up [services...]` | Bring a stack up. With no args: the whole stack; with service names: only those (deps auto-included). `-p/--profile` activates compose profiles; `--base` borrows the rest from a base stack (see [Base stacks](base-stacks.md)). Tilt runner: foreground (`-d` to detach). Compose runner: detached; `--build` rebuilds. `--json` prints `{slug,urls[],...}`; `--wait` blocks until serving (`--wait-timeout`, default 90s). |
| `lane down` | Tear down the stack; `--volumes` also removes named volumes. |
| `lane restart [services...]` | Recreate (down then up). |
| `lane ls` | List running stacks; `--json` for machine output. |
| `lane view` | Interactive control panel (TTY); `--plain` / piped prints a static snapshot. |
| `lane proxy up\|down\|status` | Manage the shared Traefik proxy. |
| `lane tls enable\|disable\|status` | Optional HTTPS via mkcert. |
| `lane init` | Scaffold `.lane.toml` from your compose. |
| `lane open` / `lane logs` | Open a stack's URL / tail its logs. |
| `lane doctor` | Preflight checks (Docker, Compose ≥ 2.20, `*.localhost`). |
| `lane skills` | Show the agent integrations lane can install (Claude Code skill, Cursor rule, AGENTS.md section) and whether each is already present. `--json`; `--global` shows global-config targets. |
| `lane teach [claude\|cursor\|agents...]` | Install those integrations into the current project. No args → auto-detect the harnesses in use (installs all three if none detected). `--claude`/`--cursor`/`--agents-md` select explicitly. `--global` installs the Claude skill to user config; for Cursor it prints the rule to paste into Settings → Rules (global Cursor rules are UI-only). `--dry-run` previews; `--json` for machine output. |

Every HTTP service is reachable at `<slug>-<service>.localhost` automatically —
see [Selecting & reaching services](selecting-services.md).

Global flags: `-C/--path <dir>` (project directory, default: cwd), `--slug`
(override identity), `--dry-run`, `-v/--verbose`.
Exit codes: `0` success (incl. already-running); `1` error.
