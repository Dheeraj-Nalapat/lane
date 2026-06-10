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

Every HTTP service is reachable at `<slug>-<service>.localhost` automatically —
see [Selecting & reaching services](selecting-services.md).

Global flags: `-C/--path <dir>` (project directory, default: cwd), `--slug`
(override identity), `--dry-run`, `-v/--verbose`.
Exit codes: `0` success (incl. already-running); `1` error.
