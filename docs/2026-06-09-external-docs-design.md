# lane — External Docs & Recipes (E) — Design Spec

**Date:** 2026-06-09
**Status:** Design complete — ready for implementation planning.
**Sub-project:** E of the generic-release effort.

## Context

The README covers reference (install, commands, HTTPS, agents, troubleshooting).
What's missing for newcomers is a narrative **getting-started** and **per-project
recipes**. These are standalone markdown so sub-project F (website) can source
them unchanged.

## Goal

A "first 10 minutes" tutorial plus three onboarding recipes (plain compose, Tilt,
frontend HMR), all accurate against the shipped CLI.

## Scope

- `docs/guide/getting-started.md` — zero → two stacks tutorial.
- `docs/guide/recipes/compose.md` — plain docker-compose project (no Tiltfile).
- `docs/guide/recipes/tilt.md` — Tilt project (gated shim); links to the ReMind
  worked example.
- `docs/guide/recipes/frontend-hmr.md` — Vite **and** Next.js dev servers behind
  the proxy.
- `docs/guide/README.md` — index/reading order.
- README gains a short "Guides" pointer.

Non-goals: Rails/other framework recipes; reference-doc rewrites; the website
itself (F). No code changes.

## Verified facts (so the docs don't drift)

All commands checked against the built `lane`:
- `lane init`, `lane up` (`-d`, `--build`, `--json`, `--wait`, `--wait-timeout`),
  `lane down [--volumes]`, `lane ls [--json]`, `lane view [--plain]`,
  `lane restart`, `lane proxy up|down|status`, `lane doctor`,
  `lane tls enable|disable|status`.
- Slug: flag > `LANE_SLUG` > `.lane.toml` name (+ git-worktree suffix) > dir name.
- Compose runner is auto-selected when there's no Tiltfile; built-image isolation
  is automatic for compose `build:` services.

**Frontend HMR — tested through lane (2026-06-09):**
- **Vite:** needs the env-gated `vite.config.ts` block — `server.host:'0.0.0.0'`,
  `server.allowedHosts:['.localhost']` (Vite blocks unknown Host headers),
  `server.hmr.clientPort:80`, and a `/api` proxy `target` from
  `process.env.LANE_API_TARGET`. (Verified earlier in HTTPS/runner work.)
- **Next.js (v16, Turbopack):** **works behind the proxy with no special
  config** — just run `next dev -H 0.0.0.0 -p <port>` and route that port. The
  page served `200` through `*.localhost`, and the HMR websocket
  (`/_next/webpack-hmr`) completed a `101` upgrade through Traefik. No
  `allowedDevOrigins` was required at this version. The recipe will note: if an
  older/other Next version logs a cross-origin dev warning, add
  `allowedDevOrigins: ['*.localhost']` to `next.config.js`.

## Content outline

### getting-started.md
Narrative, copy-pasteable: prerequisites (Docker/Compose/Tilt-optional/git) →
`lane doctor` → in a project, `lane init` then `lane up` → open
`http://<slug>.localhost` and `lane ls`/`lane view` → `git worktree add` a second
checkout → `lane up` there → **both reachable at once** → `lane down`. Closes with
links to the recipes, the HTTPS section, and the agent section.

### recipes/compose.md
The common case: a repo with only `docker-compose.yml`, **no Tiltfile**.
`lane init` writes `.lane.toml`; `lane up` uses the compose runner (detached);
built images are isolated per slug automatically. Shows the `.lane.toml`, the
`lane up`/`--json` output, and `lane down`. Notes `-d` is a no-op here and
`--build` forces a rebuild.

### recipes/tilt.md
A Tilt project: in the `--docker` branch, the gated shim
(`config.define_bool("docker")`, append `LANE_COMPOSE_OVERRIDE`,
`docker_compose(..., project_name=os.getenv("LANE_SLUG"))`, optional per-slug
`tag`). Keeps live-reload + the Tilt dashboard (`tilt-<slug>.localhost`). Links
to `docs/onboarding-remind.md` as the concrete worked example (kept as-is, not
duplicated).

### recipes/frontend-hmr.md
Two subsections:
- **Vite** — the env-gated `vite.config.ts` block (the verified one), explaining
  each line (allowedHosts, hmr.clientPort, api target) and that it's inert
  outside lane.
- **Next.js** — the verified minimal recipe: a compose/Tilt service running
  `next dev -H 0.0.0.0 -p <port>`, routed via `.lane.toml`; HMR works as-is. Plus
  the `allowedDevOrigins` fallback note for versions that warn.
Frames the general principle: a dev server must bind `0.0.0.0` and (if it
host-checks, like Vite) allow the `*.localhost` host; HMR rides the same `:80`
through Traefik.

### guide/README.md
Index: start with getting-started, then the recipe matching your project, then
HTTPS/agents in the main README.

## Testing / verification

- No unit tests (docs only). The plan's verification step **re-runs each shown
  command's `--help`/dry-run against the built binary** to confirm flags/names
  exist, and checks that every internal markdown link resolves.
- The Vite/Next specifics are already empirically verified (above); the recipes
  restate the tested config.

## Backward compatibility

Additive docs; no code or behavior change. The existing `onboarding-remind.md`
stays as the Tilt worked example, referenced (not copied) by `recipes/tilt.md`.
