# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project aims
to follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.2.2] - 2026-06-16

### Fixed
- **`lane --version` now reports the real version.** Previously it always
  printed `dev` unless built by GoReleaser. It now falls back to the binary's
  embedded build info — the module version for `go install`, or the VCS revision
  for local builds — when the release `-ldflags` version is unset.

## [0.2.1] - 2026-06-15

### Added
- **Self-installing agent skills — `lane teach` / `lane skills`:** `lane skills`
  shows the agent integrations lane can install (Claude Code skill, Cursor rule,
  AGENTS.md section) and whether each is present; `lane teach` installs them. With
  no arguments `lane teach` auto-detects the harnesses in use (`.claude/`,
  `.cursor/`, `AGENTS.md`) and installs for those, or all three if none are
  detected. Content is embedded in the binary. `--global` installs the Claude
  skill to user config (Cursor global rules are UI-only, so it prints the rule to
  paste into Settings → Rules); `--dry-run` previews; `--json` for machine output.
  AGENTS.md edits are merged into a `<!-- lane:start -->`/`<!-- lane:end -->`
  block, preserving the rest of the file.

## [0.2.0] - 2026-06-10

### Added
- **Selective / minimal bring-up:** `lane up [services...]` brings up only the
  named services (their `depends_on` is auto-included); `-p/--profile` activates
  Docker Compose profiles. Plain `lane up` still brings up the whole stack.
- **Per-service auto-routing:** every HTTP service is reachable at
  `<slug>-<service>.localhost` (covered by the existing `*.localhost` TLS cert),
  alongside explicit `[[routes]]`. `[[routes]]` is now **optional**; a new
  `[autoroute]` block in `.lane.toml` disables it or excludes services.
- **Base-borrowing — `lane up <svc> --base`:** run the changed services fresh in
  a worktree and borrow everything else from a running **base** stack of the same
  project (compose runner), instead of booting a full copy. `--json` reports
  `base`, `fresh`, and `borrowed`.
- `lane up --json` now reports per-service `running` status; `--wait` waits only
  on services that actually started.
- `lane.project` identity label on every stack (powers base discovery).

### Changed
- **Breaking:** the project directory moved from a positional `[path]` to the
  global `-C`/`--path` flag (default: current directory) on `up`/`down`/`logs`/
  `restart`. The positional arguments to `up`/`restart` are now **service names**.

## [0.1.0] - 2026-06-09

### Added
- Shared Traefik proxy on the `lane` network; routes `Host(<slug>.localhost)` to
  each stack with zero published host ports.
- Non-invasive generated compose override (`!reset` host ports + container names,
  Traefik + `lane.*` labels).
- Slug derivation: flag > `LANE_SLUG` > `.lane.toml` name (+ git-worktree suffix)
  > directory name.
- Runners: **tilt** (live-reload + dashboard, requires a Tiltfile shim) and
  **compose** (zero-config, no Tiltfile), auto-detected; `runner` override in
  `.lane.toml`.
- CLI: `up` (`-d`, `--build`), `down`, `ls`, `view` (`--watch`), `proxy`,
  `doctor`, `init`, `open`, `logs`; global `--slug`, `--dry-run`.
- Cross-platform static binaries via GoReleaser (linux/darwin/windows ×
  amd64/arm64).
- Optional HTTPS: `lane tls enable|disable|status` serves trusted
  `https://*.localhost` via mkcert (alongside HTTP; no redirect). mkcert is not
  a hard dependency.
- Help website (MkDocs Material): a custom landing page with a CLI showcase plus
  the guides, deployable to GitHub Pages.
- Agent integration: `lane up --json`/`--wait` and `lane ls --json` for
  machine-driven use; race-safe parallel `up` (locked proxy bring-up); a Claude
  Code skill (packaged as a plugin + marketplace) and a Cursor rule documenting
  the worktree → `lane up --wait --json` → test → `lane down` parallel loop.
- `lane view` is now an interactive control panel (select a stack; open / logs /
  restart / down), auto-refreshing; falls back to a static snapshot when piped
  (`--plain` to force). Replaces the `--watch` flag.
- Robustness: actionable preflight errors (Docker not running, Compose < 2.20,
  host-port conflicts); `lane up` no-ops when the stack is already running;
  `lane restart`; `lane down --volumes`; per-slug built-image isolation
  (automatic for the compose runner).
