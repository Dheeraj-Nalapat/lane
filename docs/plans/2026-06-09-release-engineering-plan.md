# Release Engineering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the lane repo release-ready and publicly consumable — correct module path, MIT license, CI on push/PR, and contributor/release docs — all verified locally, nothing published.

**Architecture:** Prepare-only. Fix the placeholder module path to the real GitHub username, add `LICENSE`/`CONTRIBUTING`/`CHANGELOG`/`RELEASING`, a `ci.yml` workflow, and README badges. No code logic changes beyond the mechanical import rename. The owner pushes/tags later under their personal account per `RELEASING.md`.

**Tech Stack:** Go 1.22 (module path), GitHub Actions (CI), GoReleaser (existing release workflow). No new dependencies.

Spec: `docs/2026-06-09-release-engineering-design.md`.

---

## File Structure

```
go.mod + all *.go imports        module path dheerajnalapat → dheeraj-nalapat
.goreleaser.yaml, install.sh      owner/REPO → dheeraj-nalapat
LICENSE                           NEW — MIT
CONTRIBUTING.md                   NEW
CHANGELOG.md                      NEW
RELEASING.md                      NEW
.github/workflows/ci.yml          NEW — gofmt/vet/test/build on push/PR
README.md                         badges + path references
```

These are independent; verification is local commands, not unit tests.

---

### Task 1: Correct the module path (`dheerajnalapat` → `dheeraj-nalapat`)

**Files:**
- Modify: `go.mod`, every `*.go` import, `.goreleaser.yaml`, `install.sh`, `README.md`, `docs/*`

- [ ] **Step 1: Replace the path across all tracked text files**

Run from the repo root:
```bash
git ls-files | while read -r f; do
  case "$f" in
    *.go|*.mod|*.yaml|*.yml|*.sh|*.md|*.toml) sed -i 's#dheeraj-nalapat/lane#dheeraj-nalapat/lane#g' "$f" ;;
  esac
done
# .goreleaser.yaml's Homebrew tap owner is a BARE name (no /lane), so patch it separately:
sed -i 's/owner: dheerajnalapat$/owner: dheeraj-nalapat/' .goreleaser.yaml
```

> The path replace is scoped to `dheeraj-nalapat/lane` (not bare `dheerajnalapat`)
> so it cannot corrupt home-dir paths like `~/project/...` in
> the docs. The bare `owner:` line is handled by the second `sed`.

- [ ] **Step 2: Verify the module line + no stale path remain**

Run:
```bash
head -1 go.mod
git ls-files | xargs grep -nI 'dheeraj-nalapat/lane' 2>/dev/null | wc -l
```
Expected: `module github.com/dheeraj-nalapat/lane`; residual count `0`.

- [ ] **Step 3: Build + test under the new path**

Run: `go build ./... && go test ./... && go vet ./...`
Expected: builds, all tests pass, vet clean.

- [ ] **Step 4: Reinstall the binary (keeps your `lane` current)**

Run: `go build -o ~/.local/bin/lane . && hash -r`
Expected: no output; `lane` still runs.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "chore: set module path to github.com/dheeraj-nalapat/lane"
```

---

### Task 2: `LICENSE` (MIT)

**Files:**
- Create: `LICENSE`

- [ ] **Step 1: Write the MIT license**

`LICENSE`:
```
MIT License

Copyright (c) 2026 Dheeraj Nalapat

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 2: Commit**

```bash
git add LICENSE
git commit -m "docs: add MIT license"
```

---

### Task 3: `CONTRIBUTING.md`

**Files:**
- Create: `CONTRIBUTING.md`

- [ ] **Step 1: Write it**

`CONTRIBUTING.md`:
```markdown
# Contributing to lane

Thanks for helping improve lane! lane is a small Go CLI; see
[`docs/DEVELOPMENT.md`](docs/DEVELOPMENT.md) for architecture and internals.

## Quick start

```bash
go build ./...
go test ./...
go vet ./...
gofmt -l .          # must print nothing
go build -o ~/.local/bin/lane .   # install your build
```

## Pull requests

- Keep each PR to one logical change.
- CI must be green: `gofmt`, `go vet`, and `go test ./...` all pass.
- Add/adjust tests for behavior changes (table tests preferred — see existing
  `internal/*/*_test.go`).
- Update `CHANGELOG.md` under `## [Unreleased]`.

## Linting

CI gates on `gofmt` + `go vet` only. `golangci-lint run` is welcome locally for
stricter checks but is **not** required to merge.

## Commit messages

Conventional-ish prefixes (`feat:`, `fix:`, `docs:`, `chore:`) keep the
changelog easy to assemble.
```

- [ ] **Step 2: Commit**

```bash
git add CONTRIBUTING.md
git commit -m "docs: add CONTRIBUTING guide"
```

---

### Task 4: `CHANGELOG.md`

**Files:**
- Create: `CHANGELOG.md`

- [ ] **Step 1: Write it**

`CHANGELOG.md`:
```markdown
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
```

- [ ] **Step 2: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: add CHANGELOG (Keep a Changelog)"
```

---

### Task 5: `RELEASING.md`

**Files:**
- Create: `RELEASING.md`

- [ ] **Step 1: Write the runbook**

`RELEASING.md`:
```markdown
# Releasing lane

lane is published under the personal GitHub account **`Dheeraj-Nalapat`**.
(Do not use a work account for this.)

## One-time setup

```bash
# 1. Create the repos on GitHub (web UI or `gh` logged in as Dheeraj-Nalapat):
#      Dheeraj-Nalapat/lane           (public)
#      Dheeraj-Nalapat/homebrew-lane  (public; the Homebrew tap)

# 2. Wire the remote and push:
git remote add origin git@github.com:Dheeraj-Nalapat/lane.git
git push -u origin master
```

GoReleaser's Homebrew step pushes a formula to the tap repo, which needs a token
with write access to `homebrew-lane`. Add it as the `HOMEBREW_TAP_GITHUB_TOKEN`
(or a PAT) repo secret if the default `GITHUB_TOKEN` can't write across repos.

## Cutting a release

```bash
# Update CHANGELOG: rename [Unreleased] → [vX.Y.Z] - <date>, add a fresh [Unreleased].
git tag v0.1.0
git push origin v0.1.0      # triggers .github/workflows/release.yml → GoReleaser
```

GoReleaser then builds the cross-platform binaries, creates the GitHub Release
with archives + checksums, and updates the Homebrew tap.

## Versioning

Semantic Versioning. Stay on `0.x` while the CLI surface and `.lane.toml`
schema may still change; cut `v1.0.0` once they're stable.

## Verify a release locally first

```bash
goreleaser build --snapshot --clean   # builds all targets, no publish
goreleaser check                       # passes once a git remote exists
```
```

- [ ] **Step 2: Commit**

```bash
git add RELEASING.md
git commit -m "docs: add release runbook"
```

---

### Task 6: CI workflow

**Files:**
- Create: `.github/workflows/ci.yml`

- [ ] **Step 1: Write the workflow**

`.github/workflows/ci.yml`:
```yaml
name: ci
on:
  push:
    branches: [master, main]
  pull_request:
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: "1.22" }
      - name: gofmt
        run: |
          unformatted=$(gofmt -l .)
          if [ -n "$unformatted" ]; then echo "gofmt needed on:"; echo "$unformatted"; exit 1; fi
      - run: go vet ./...
      - run: go test ./...
      - run: go build ./...
```

- [ ] **Step 2: Verify the same steps pass locally (what CI will run)**

Run:
```bash
test -z "$(gofmt -l .)" && echo "gofmt clean" || (gofmt -l .; echo "FIX FORMATTING"; false)
go vet ./... && go test ./... && go build ./...
```
Expected: `gofmt clean`, vet/test/build all succeed.

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "ci: run gofmt/vet/test/build on push and PR"
```

---

### Task 7: README badges

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add badges under the title**

In `README.md`, immediately after the `# lane 🛣️` title line, insert a blank
line then this badge block:
```markdown
[![CI](https://github.com/Dheeraj-Nalapat/lane/actions/workflows/ci.yml/badge.svg)](https://github.com/Dheeraj-Nalapat/lane/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Dheeraj-Nalapat/lane)](https://github.com/Dheeraj-Nalapat/lane/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
```

> Badges render once the repo is public and CI has run; before that they show
> "no status" / "no releases", which is expected pre-publish.

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add CI/release/license badges to README"
```

---

### Task 8: Final verification (local, no publishing)

**No files. Verification gate.**

- [ ] **Step 1: Full local check**

Run:
```bash
gofmt -l .                      # empty
go vet ./... && go test ./...   # clean + pass
go build -o ~/.local/bin/lane . # installs current build
goreleaser build --snapshot --clean   # all 6 targets build; binary "lane"
```
Expected: gofmt empty; vet/test pass; snapshot build succeeds with `lane` binaries in `dist/`.

- [ ] **Step 2: Confirm no stale path and no accidental publish**

Run:
```bash
git ls-files | xargs grep -nI 'dheeraj-nalapat/lane' 2>/dev/null | wc -l   # 0
grep -n 'owner: dheerajnalapat$' .goreleaser.yaml | wc -l                  # 0 (bare owner fixed)
git remote -v                                                              # empty (prepare-only)
```
Expected: both counts `0`; no remote configured (nothing was pushed).

- [ ] **Step 3: Confirm release files present**

Run: `ls LICENSE CONTRIBUTING.md CHANGELOG.md RELEASING.md .github/workflows/ci.yml`
Expected: all listed.

---

## Self-review checklist (for the implementer)

- [ ] Module path is `github.com/dheeraj-nalapat/lane` everywhere; `go test ./...` green.
- [ ] LICENSE/CONTRIBUTING/CHANGELOG/RELEASING/ci.yml exist; README has badges.
- [ ] `.goreleaser.yaml` `brews.repository.owner` and `install.sh` `REPO` read `dheeraj-nalapat`.
- [ ] Nothing pushed; no remote added; the work `gh` account was not used.
