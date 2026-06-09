# Help Website (F) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A MkDocs Material help site with a custom static landing page (hero + CLI terminal showcase) over the existing `docs/guide/` markdown, deployable to GitHub Pages.

**Architecture:** `mkdocs.yml` (`docs_dir: docs/guide`, `custom_dir: overrides`); a custom `overrides/home.html` landing (with real captured CLI output in CSS terminal frames) selected by `index.md` front matter; new `commands/https/agents` doc pages distilled from the README; a GH Actions → Pages workflow. **Prepare-only:** local `mkdocs build --strict` is the gate; nothing is deployed.

**Tech Stack:** MkDocs + Material (Python, pip), HTML/CSS, GitHub Actions.

Spec: `docs/2026-06-09-help-website-design.md`.

---

## File Structure

```
mkdocs.yml                       site config + nav
requirements-docs.txt            mkdocs-material pin
overrides/home.html              custom landing template
docs/guide/index.md              home stub (front matter: template: home.html)
docs/guide/assets/lane.css       landing + terminal-frame styles (extra_css)
docs/guide/commands.md           NEW (from README command table)
docs/guide/https.md              NEW (from README HTTPS)
docs/guide/agents.md             NEW (from README agents)
.github/workflows/docs.yml       build + deploy to Pages
.gitignore                       + .venv-docs/, /site/
README.md, CHANGELOG.md          publish runbook pointer + entry
(edits) docs/guide/getting-started.md, docs/guide/recipes/tilt.md  repoint out-of-tree links
```

---

### Task 1: Site scaffold (mkdocs.yml, requirements, gitignore)

**Files:** Create `mkdocs.yml`, `requirements-docs.txt`; modify `.gitignore`

- [ ] **Step 1: `requirements-docs.txt`**

```
mkdocs-material>=9.5
```

- [ ] **Step 2: `mkdocs.yml`**

```yaml
site_name: lane
site_description: Parallel, port-conflict-free dev stacks.
site_url: https://dheeraj-nalapat.github.io/lane/
repo_url: https://github.com/Dheeraj-Nalapat/lane
docs_dir: docs/guide
exclude_docs: |
  README.md
theme:
  name: material
  custom_dir: overrides
  palette:
    - scheme: default
      primary: cyan
      accent: cyan
      toggle:
        icon: material/weather-night
        name: Dark mode
    - scheme: slate
      primary: cyan
      accent: cyan
      toggle:
        icon: material/weather-sunny
        name: Light mode
  features:
    - navigation.top
    - content.code.copy
    - search.suggest
extra_css:
  - assets/lane.css
markdown_extensions:
  - admonition
  - pymdownx.highlight
  - pymdownx.superfences
nav:
  - Home: index.md
  - Getting started: getting-started.md
  - Recipes:
    - Plain compose: recipes/compose.md
    - Tilt: recipes/tilt.md
    - Frontend HMR: recipes/frontend-hmr.md
  - HTTPS: https.md
  - Agents: agents.md
  - Command reference: commands.md
```

- [ ] **Step 3: gitignore**

Append to `.gitignore`:
```
.venv-docs/
/site/
```

- [ ] **Step 4: Commit**

```bash
git add mkdocs.yml requirements-docs.txt .gitignore
git commit -m "docs(site): mkdocs material scaffold"
```

---

### Task 2: Doc pages (commands / https / agents) + link repointing

**Files:** Create `docs/guide/commands.md`, `docs/guide/https.md`, `docs/guide/agents.md`; modify `docs/guide/getting-started.md`, `docs/guide/recipes/tilt.md`

- [ ] **Step 1: `docs/guide/commands.md`**

```markdown
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
```

- [ ] **Step 2: `docs/guide/https.md`**

```markdown
# HTTPS (optional)

lane serves HTTP by default. For trusted `https://<slug>.localhost` (secure
cookies, HTTPS-only APIs), install [mkcert](https://github.com/FiloSottile/mkcert)
and enable TLS:

```bash
mkcert -install     # one-time; adds a local CA to your trust store
lane tls enable     # generates a wildcard cert, restarts the proxy on :443
lane up             # re-up running stacks to add their HTTPS route
```

Both `http://` and `https://` then serve every stack. `lane tls status` shows
state; `lane tls disable` returns to HTTP-only. mkcert is **not** required for
normal use — only to enable HTTPS. Nested hosts (`api.<slug>.localhost`) aren't
covered by the wildcard cert.
```

- [ ] **Step 3: `docs/guide/agents.md`**

```markdown
# Using lane with coding agents (parallel testing)

lane gives each git worktree an isolated, port-conflict-free stack, so multiple
agents (Claude, Cursor, …) can test in parallel. The loop:

```bash
lane up --wait --json   # isolated stack for this worktree; waits until serving; prints {slug, urls[]}
# ...run tests against the returned url...
lane down
```

`--json` prints machine-readable output on stdout (human logs on stderr); exit
`0` = success (incl. already-running), `1` = error. `lane ls --json` lists
stacks. Parallel `lane up`s are race-safe (the shared proxy bring-up is locked).

**Skill files:** a Claude Code skill (installable as a plugin —
`/plugin marketplace add Dheeraj-Nalapat/lane`) and a Cursor rule live under
`agent/` in the repo.
```

- [ ] **Step 4: Repoint out-of-tree links (so `--strict` passes)**

In `docs/guide/getting-started.md`, replace the "Next steps" README link:
```markdown
- HTTPS, and driving lane from coding agents (parallel testing): see the
  [README](../../README.md).
```
with:
```markdown
- [HTTPS](https.md) and [using lane with agents](agents.md) (parallel testing).
```

In `docs/guide/recipes/tilt.md`, replace the worked-example link:
```markdown
See [`docs/onboarding-remind.md`](../../onboarding-remind.md) — the ReMind
```
with:
```markdown
See [the ReMind onboarding walkthrough](https://github.com/Dheeraj-Nalapat/lane/blob/main/docs/onboarding-remind.md) — the ReMind
```

- [ ] **Step 5: Commit**

```bash
git add docs/guide/commands.md docs/guide/https.md docs/guide/agents.md docs/guide/getting-started.md docs/guide/recipes/tilt.md
git commit -m "docs(site): command/https/agents pages; repoint out-of-tree links"
```

---

### Task 3: Capture real CLI output for the showcase

**Files:** none committed (scratch). Requires Docker.

- [ ] **Step 1: Capture authentic output**

```bash
go build -o ./bin/lane .
./bin/lane proxy up >/dev/null 2>&1
mkdir -p /tmp/lane-shot && cd /tmp/lane-shot
printf 'services:\n  web:\n    image: traefik/whoami\n' > docker-compose.yml
printf 'name = "demo"\ncompose_file = "docker-compose.yml"\n[[routes]]\nservice = "web"\nport = 80\n' > .lane.toml
/home/dheerajnalapat/project/lane/bin/lane up --wait --json 2>/dev/null    # copy this JSON
/home/dheerajnalapat/project/lane/bin/lane ls                              # copy this table
# teardown
/home/dheerajnalapat/project/lane/bin/lane down >/dev/null 2>&1
cd / && rm -rf /tmp/lane-shot
```

- [ ] **Step 2: Record the captured text**

Paste the real `up --json` JSON and the `ls` table into the Task 4 `home.html`
terminal frames. For the `lane view` panel frame, use the block-LANE logo +
two example stacks rendered in the known `RenderPanel` layout (logo, `proxy ●`,
two slugs + routes, footer keys), labeled as the interactive panel.

(No commit — this is input for Task 4.)

---

### Task 4: Landing page (index.md + home.html + lane.css)

**Files:** Create/replace `docs/guide/index.md`; create `overrides/home.html`, `docs/guide/assets/lane.css`

- [ ] **Step 1: `docs/guide/index.md`** (home stub)

```markdown
---
template: home.html
hide:
  - navigation
  - toc
---
```

- [ ] **Step 2: `overrides/home.html`**

```html
{% extends "main.html" %}

{% block content %}
<section class="lane-hero">
  <h1 class="lane-name">lane</h1>
  <p class="lane-tag">Parallel, port-conflict-free dev stacks.</p>
  <p class="lane-sub">Run many projects — and many git worktrees of one project —
     at once. Each gets an isolated stack at a friendly <code>*.localhost</code> URL.</p>
  <div class="lane-cta">
    <a class="md-button md-button--primary" href="getting-started/">Get started</a>
    <a class="md-button" href="https://github.com/Dheeraj-Nalapat/lane">GitHub</a>
  </div>
  <pre class="lane-install"><code>brew install Dheeraj-Nalapat/lane/lane</code></pre>
</section>

<section class="lane-cards">
  <div class="lane-card"><h3>Run many at once</h3><p>No host ports published — a shared proxy routes by hostname, so stacks never collide.</p></div>
  <div class="lane-card"><h3>Friendly URLs</h3><p>Each stack at <code>&lt;slug&gt;.localhost</code>; worktrees get distinct slugs automatically.</p></div>
  <div class="lane-card"><h3>Agent-drivable</h3><p><code>lane up --wait --json</code> — machine-readable, race-safe parallel testing for Claude/Cursor.</p></div>
</section>

<section class="lane-showcase">
  <div class="lane-term">
    <div class="lane-term-bar"><span></span><span></span><span></span><i>lane up --wait --json</i></div>
    <pre><code>{
  "slug": "demo",
  "runner": "compose",
  "tls": false,
  "urls": [{ "service": "web", "host": "demo.localhost", "url": "http://demo.localhost" }]
}</code></pre>
  </div>

  <div class="lane-term">
    <div class="lane-term-bar"><span></span><span></span><span></span><i>lane view</i></div>
<pre><code>██╗      █████╗ ███╗   ██╗███████╗
██║     ██╔══██╗████╗  ██║██╔════╝     🏁  parallel dev stacks
██║     ███████║██╔██╗ ██║█████╗
██║     ██╔══██║██║╚██╗██║██╔══╝       proxy ● up    tls ○ off
███████╗██║  ██║██║ ╚████║███████╗
╚══════╝╚═╝  ╚═╝╚═╝  ╚═══╝╚══════╝
──────────────────────────────────┬───────────────────────
<span class="g">▸ remind</span>        ● running        │ remind  →  http://remind.localhost
  demo          ● running        │   ✓ remind.localhost → remind-ui
                                  │   ✓ tilt-remind.localhost → tilt
──────────────────────────────────┴───────────────────────
 ↑/↓ select  o open  l logs  r restart  x down  q quit</code></pre>
  </div>
</section>
{% endblock %}
```

> Replace the `up --json` block and (if desired) the panel with the **real
> captured output from Task 3**; the values above are the expected shape.

- [ ] **Step 3: `docs/guide/assets/lane.css`**

```css
.lane-hero { text-align: center; padding: 3.5rem 1rem 2rem; }
.lane-name { font-size: 4rem; font-weight: 800; letter-spacing: .08em; margin: 0;
  color: var(--md-primary-fg-color); }
.lane-tag { font-size: 1.4rem; font-weight: 600; margin: .25rem 0 .75rem; }
.lane-sub { max-width: 40rem; margin: 0 auto 1.5rem; opacity: .85; }
.lane-cta { display: flex; gap: .75rem; justify-content: center; margin-bottom: 1.25rem; }
.lane-install { display: inline-block; }
.lane-install code { padding: .5rem 1rem; }

.lane-cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 1rem; max-width: 60rem; margin: 1rem auto 2.5rem; padding: 0 1rem; }
.lane-card { border: 1px solid var(--md-default-fg-color--lightest); border-radius: .6rem;
  padding: 1rem 1.2rem; }
.lane-card h3 { margin: 0 0 .4rem; }

.lane-showcase { display: grid; gap: 1.25rem; max-width: 60rem; margin: 0 auto 3rem; padding: 0 1rem; }
.lane-term { background: #0d1117; border-radius: .6rem; overflow: hidden;
  box-shadow: 0 8px 30px rgba(0,0,0,.25); }
.lane-term-bar { display: flex; align-items: center; gap: .4rem; padding: .5rem .8rem;
  background: #161b22; }
.lane-term-bar span { width: 12px; height: 12px; border-radius: 50%; background: #3a4250; }
.lane-term-bar span:nth-child(1){ background:#ff5f56 } .lane-term-bar span:nth-child(2){ background:#ffbd2e } .lane-term-bar span:nth-child(3){ background:#27c93f }
.lane-term-bar i { margin-left: .6rem; color: #8b949e; font-style: normal; font-size: .8rem; }
.lane-term pre { margin: 0; padding: 1rem; background: #0d1117; color: #c9d1d9;
  overflow-x: auto; }
.lane-term code { color: inherit; background: none; font-size: .82rem; line-height: 1.4; }
.lane-term .g { color: #3fb950; font-weight: 700; }
```

- [ ] **Step 4: Commit**

```bash
git add docs/guide/index.md overrides/home.html docs/guide/assets/lane.css
git commit -m "docs(site): custom landing page with CLI terminal showcase"
```

---

### Task 5: Deploy workflow + publish runbook + CHANGELOG

**Files:** Create `.github/workflows/docs.yml`; modify `README.md`, `CHANGELOG.md`

- [ ] **Step 1: `.github/workflows/docs.yml`**

```yaml
name: docs
on:
  push:
    branches: [main]
permissions:
  contents: read
  pages: write
  id-token: write
jobs:
  build-deploy:
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deploy.outputs.page_url }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-python@v5
        with: { python-version: "3.12" }
      - run: pip install -r requirements-docs.txt
      - run: mkdocs build --strict
      - uses: actions/upload-pages-artifact@v3
        with: { path: site }
      - id: deploy
        uses: actions/deploy-pages@v4
```

- [ ] **Step 2: README publish runbook pointer**

In `README.md`, under the "Guides" pointer (or near the bottom), add:
```markdown
The guides also build into a website (MkDocs Material). Locally:
`pip install -r requirements-docs.txt && mkdocs serve`. It auto-deploys to GitHub
Pages on push to `main` once Pages is enabled (Settings → Pages → Source: GitHub
Actions).
```

- [ ] **Step 3: CHANGELOG**

Under `## [Unreleased]` → `### Added`, append:
```markdown
- Help website (MkDocs Material): a custom landing page with a CLI showcase plus
  the guides, deployable to GitHub Pages.
```

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/docs.yml README.md CHANGELOG.md
git commit -m "docs(site): Pages deploy workflow + publish runbook"
```

---

### Task 6: Build gate (local `mkdocs build --strict`)

**No new files. Verification.**

- [ ] **Step 1: Build strict in a venv**

```bash
cd /home/dheerajnalapat/project/lane
python3 -m venv .venv-docs && . .venv-docs/bin/activate
pip install -r requirements-docs.txt
mkdocs build --strict
deactivate
```
Expected: build succeeds with **no warnings/errors** (strict fails on broken
links or pages missing from nav).

- [ ] **Step 2: Spot-check the built landing**

```bash
grep -q 'lane-hero' site/index.html && grep -q 'lane-term' site/index.html && echo "landing rendered with hero + terminal frames"
grep -q 'assets/lane.css' site/index.html && echo "css linked"
```
Expected: both echoes print.

- [ ] **Step 3: Fix any strict failures, then re-run; commit fixes**

```bash
git commit -am "docs(site): fix mkdocs strict warnings" || echo "nothing to fix"
```

---

## Final verification

- [ ] `mkdocs build --strict` succeeds (nav complete, no broken links).
- [ ] `site/index.html` contains the hero + terminal-frame showcase + linked CSS.
- [ ] The `up --json` terminal frame shows the **real captured** output (Task 3).
- [ ] `commands/https/agents` pages present; out-of-tree links repointed.
- [ ] `.venv-docs/` and `site/` are gitignored; no code changed; `git status` clean.
- [ ] Nothing deployed (prepare-only); runbook documents enabling Pages.
