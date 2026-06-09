# lane — Help Website (F) — Design Spec

**Date:** 2026-06-09
**Status:** Design complete — ready for implementation planning.
**Sub-project:** F of the generic-release effort (final).

## Context

lane is feature-complete and documented (`docs/guide/`). F publishes a help
website: a **designed static landing page** plus the markdown guides, on GitHub
Pages. Like sub-project D, this is **prepare-only** — build/verify the site
locally and document the publish step; the owner enables Pages when the repo is
public.

## Goal

A polished landing page (hero + pitch + CLI showcase) backed by the existing
markdown docs, built with MkDocs Material and deployable to GitHub Pages.

## Decisions

| Item | Decision |
|---|---|
| Generator | **MkDocs + Material** |
| Landing page | **Custom** static frame via a Material template override (not rendered markdown) |
| Docs pages | The existing `docs/guide/` markdown (single source of truth) |
| CLI showcase | **CSS "terminal window" components rendering real captured `lane` output** (not PNGs; can swap PNGs later) |
| Accent | Cyan (matches the TUI), dark/light toggle, search, code-copy |
| Hosting | GitHub Actions → Pages; `site_url: https://dheeraj-nalapat.github.io/lane/` |
| Publish | **Prepare-only** — local `mkdocs build --strict` is the gate; runbook to enable Pages |

## Non-goals

- Custom domain (later swap); blog; versioned docs; analytics; real PNG/GIF
  captures (CSS terminal frames stand in).

## Structure

```
mkdocs.yml                         site config + nav (docs_dir: docs/guide)
overrides/home.html                custom landing template (extends main.html)
overrides/assets/lane.css          landing + terminal-frame styles (extra_css)
docs/guide/index.md                home page stub (front matter: template: home.html)
docs/guide/getting-started.md      (existing)
docs/guide/recipes/*.md            (existing)
docs/guide/commands.md             NEW — command reference (from README table)
docs/guide/https.md                NEW — HTTPS (from README)
docs/guide/agents.md               NEW — parallel testing with agents (from README)
requirements-docs.txt              mkdocs-material pin (for CI + local)
.github/workflows/docs.yml         build + deploy to Pages on push to main
```

`docs_dir: docs/guide` keeps the repo's `docs/` specs/plans **out** of the site
(only `docs/guide/` is published).

**Index conflict:** `docs/guide/README.md` (the GitHub-facing guide index from
sub-project E) and the new `index.md` both map to the section index. Keep
`README.md` for GitHub browsing but exclude it from the build with MkDocs
`exclude_docs: README.md`, so `index.md` (the landing) is the site home.

**Out-of-tree links:** strict mode treats links outside `docs_dir` as broken.
The existing guide cross-links must be repointed:
- `../../README.md` → in-site pages (`https.md` / `agents.md` / `commands.md`).
- `../../onboarding-remind.md` (recipes/tilt.md) → the GitHub blob URL
  `https://github.com/Dheeraj-Nalapat/lane/blob/main/docs/onboarding-remind.md`
  (external links pass strict).

## Landing page (`overrides/home.html`)

A Material template override; `docs/guide/index.md` selects it via front matter
(`template: home.html`, `hide: [navigation, toc]`). Sections:

1. **Hero** — "lane" wordmark + tagline "parallel, port-conflict-free dev
   stacks"; a one-line install (`brew install Dheeraj-Nalapat/lane/lane` /
   `curl … | sh`); CTA buttons → **Get started** (`getting-started/`) and
   **GitHub**.
2. **Pitch** — the worktree-isolation / no-host-ports story in 2–3 sentences.
3. **Feature cards (3)** — "Run many at once", "Friendly `*.localhost` URLs",
   "Agent-drivable (`--json`/`--wait`)".
4. **CLI showcase** — terminal-window components (see below).
5. **Footer CTA** — link to guides + GitHub.

Styling is hand-written CSS in `overrides/assets/lane.css` (loaded via
`extra_css`), scoped to the home page; it does not affect the doc pages' Material
styling.

## CLI showcase (terminal frames, real output)

A `.lane-term` CSS component renders a dark terminal window (title bar with the
three dots, monospace body). The body text is **real output captured from the
built binary**, pasted into `home.html` as preformatted content:

- `lane up --wait --json` → the JSON result.
- `lane ls` → the table.
- `lane view` panel → a faithful static rendering (the block-LANE logo + a couple
  of stacks + footer keys). Since the interactive TUI can't be captured
  headlessly, this frame is hand-assembled from the known `RenderPanel` layout
  and labeled as the live panel.

ANSI colors are represented with CSS classes (green slug, dim paths) rather than
raw escape codes. Output is captured during implementation (Task: "capture CLI
output") from `lane` run against a throwaway whoami stack, so it's authentic.

## Docs pages

- Nav (in `mkdocs.yml`): Home · Getting started · Recipes (Compose / Tilt /
  Frontend HMR) · HTTPS · Agents · Command reference.
- `commands.md` / `https.md` / `agents.md` are distilled from the README so the
  site is self-contained; the README keeps its own copies (README is for the
  repo, the site for browsing).
- Existing guide cross-links that point to `../../README.md` are repointed to the
  in-site pages (e.g. `agents.md`, `https.md`) so `mkdocs build --strict` passes
  (strict fails on broken links).

## Build / publish

- **Local (the gate):**
  ```bash
  python3 -m venv .venv-docs && . .venv-docs/bin/activate
  pip install -r requirements-docs.txt
  mkdocs build --strict      # fails on broken links / nav → our accuracy gate
  mkdocs serve               # optional local preview at :8000
  ```
- **CI (`.github/workflows/docs.yml`):** on push to `main`, set up Python, `pip
  install -r requirements-docs.txt`, `mkdocs build --strict`, upload the `site/`
  artifact, deploy via `actions/deploy-pages`. Permissions: `pages: write`,
  `id-token: write`.
- **Runbook** (in the spec/README): when the repo is public, enable Pages
  (Settings → Pages → Source: GitHub Actions); the workflow then deploys on
  every push. Nothing is deployed during this sub-project.
- `.venv-docs/` and `site/` are gitignored.

## Testing / verification

- `mkdocs build --strict` succeeds (this validates nav + every internal link;
  the primary gate).
- The captured CLI output in the terminal frames is real (taken from the built
  binary, not invented).
- `home.html` renders (verified via `mkdocs serve` locally / by inspecting the
  built `site/index.html` for the hero + terminal frames).

## Backward compatibility

Additive: the site is a presentation layer over existing docs. No code change.
`docs/guide/` remains valid standalone markdown (the new `index.md` front matter
is inert outside MkDocs; the added pages are normal markdown).
