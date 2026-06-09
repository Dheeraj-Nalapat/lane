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
