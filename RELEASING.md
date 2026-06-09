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
