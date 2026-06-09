# lane — Release Engineering (prepare-only) — Design Spec

**Date:** 2026-06-09
**Status:** Design complete — ready for implementation planning.
**Sub-project:** D of the generic-release effort.

## Context

lane's core is generic (sub-projects A done: Tilt **or** plain compose). To be a
*public, consumable* tool it still lacks release hygiene: a license, a real
module path, CI that runs tests, contributor/release docs, and a changelog.

This is **sub-project D**. It is **prepare-only**: make the repo release-ready
and validate everything locally. The owner will perform the GitHub-side steps
(create the public repo, push, cut the first tag, create the Homebrew tap) later,
under their **personal** account — documented in `RELEASING.md`.

> The currently-authenticated `gh` CLI is the owner's **work** account and must
> **not** be used here. lane will be published under the personal GitHub user
> **`Dheeraj-Nalapat`**. No pushing, repo creation, or tagging happens in this
> sub-project.

Remaining sibling sub-projects (each its own cycle): B (HTTPS), C (robustness),
E (external docs), and a newly-added **F (help website, GitHub Pages)**.

## Goal

Turn the repo into a release-ready, publicly-consumable project: correct module
path, MIT license, CI on every push/PR, and the docs a contributor/releaser
needs — all verified locally, nothing published.

## Decisions

| Item | Decision |
|---|---|
| GitHub owner | Personal user **`Dheeraj-Nalapat`** |
| Module path | **`github.com/dheeraj-nalapat/lane`** (lowercase, Go convention; GitHub resolves case-insensitively) |
| License | **MIT**, "Copyright (c) 2026 Dheeraj Nalapat" |
| CI | GitHub Actions on push/PR: `gofmt` check → `go vet` → `go test ./...` → `go build ./...` (Linux) |
| Lint | `gofmt` + `go vet` only in CI; `golangci-lint` noted as optional in CONTRIBUTING (not a gate) |
| Publish | **Prepare-only** — owner pushes/tags later (public) |
| Website | Out of scope here; separate sub-project F |

## Deliverables

### 1. Module-path correction (`dheerajnalapat` → `dheeraj-nalapat`)

The repo currently uses the placeholder `github.com/dheeraj-nalapat/lane`. Update
to the real username path everywhere:
- `go.mod` module line
- every Go import (`internal/...`, `cmd`, `main.go`)
- `.goreleaser.yaml` (`brews.repository.owner`, `homepage`)
- `install.sh` (`REPO=`)
- `README.md` / `docs/*` references

Mechanical, repo-wide replace of `dheerajnalapat` → `dheeraj-nalapat`; then
`go build ./... && go test ./...` must pass.

### 2. `LICENSE`

Standard MIT text, year 2026, "Dheeraj Nalapat".

### 3. `CONTRIBUTING.md`

- Quick build/test/lint commands (link to `docs/DEVELOPMENT.md` for depth).
- PR expectations: tests + `gofmt` + `go vet` green; one logical change per PR.
- Note: `golangci-lint` is welcome but optional (not CI-gated).

### 4. `CHANGELOG.md`

[Keep a Changelog](https://keepachangelog.com) format. An `## [Unreleased]`
section summarizing what exists so far (core proxy/override/slug engine; CLI
`up/down/ls/view/proxy/doctor/init/open/logs`; Tilt + Compose runners). No
version cut yet.

### 5. `RELEASING.md`

The runbook the owner follows later (personal account):
```
git remote add origin git@github.com:Dheeraj-Nalapat/lane.git
# create the public repo on GitHub (gh repo create / web UI)
# create the tap repo: Dheeraj-Nalapat/homebrew-lane
git push -u origin master
git tag v0.1.0 && git push origin v0.1.0   # triggers .github/workflows/release.yml → GoReleaser
```
Plus: semver policy (0.x until stable), and the `GITHUB_TOKEN`/tap-token note for
the Homebrew publish step.

### 6. `.github/workflows/ci.yml`

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
      - run: test -z "$(gofmt -l .)" || (gofmt -l . && exit 1)
      - run: go vet ./...
      - run: go test ./...
      - run: go build ./...
```

### 7. README badges

Add CI / license / latest-release badges near the title (URLs use
`Dheeraj-Nalapat/lane`).

## Verification (local only)

- `gofmt -l .` → empty (clean).
- `go vet ./...` → clean; `go test ./...` → all pass; `go build ./...` → ok.
- `goreleaser build --snapshot --clean` → all targets build (binary name `lane`).
- **Note:** `goreleaser check` reports "no remote configured to list refs" until
  a git remote exists — expected pre-publish; it passes once the owner adds the
  remote. We rely on the snapshot build for local validation.

## Out of scope (explicit)

- Creating the GitHub repo, pushing, creating the `homebrew-lane` tap, cutting
  real tags — all documented in `RELEASING.md` for the owner to do later.
- Using the work `gh` account for anything.
- The help website — **sub-project F**. D leaves `docs/` untouched in structure so
  F is free to choose GitHub Pages from `/docs`, a `gh-pages` branch, or a
  separate site dir.

## Testing

No new unit tests (this sub-project adds config/docs, not Go logic). The
verification above (gofmt/vet/test/build + snapshot) is the gate. The new
`ci.yml` re-runs exactly that on every push/PR once the repo is on GitHub.
