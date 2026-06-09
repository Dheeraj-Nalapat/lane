# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project aims
to follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

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
- Robustness: actionable preflight errors (Docker not running, Compose < 2.20,
  host-port conflicts); `lane up` no-ops when the stack is already running;
  `lane restart`; `lane down --volumes`; per-slug built-image isolation
  (automatic for the compose runner).
