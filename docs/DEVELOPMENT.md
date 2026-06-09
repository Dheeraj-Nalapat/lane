# berth — Developer Guide

Internals, conventions, build/test/reinstall, and how to extend berth. For
*using* berth, see the top-level [`README.md`](../README.md). For the design
rationale, see [`docs/2026-06-08-berth-design.md`](2026-06-08-berth-design.md);
for the original task breakdown, [`docs/plans/2026-06-09-berth-implementation.md`](plans/2026-06-09-berth-implementation.md).

---

## Prerequisites

- Go 1.22+
- Docker ≥ 28 / Compose ≥ 2.20, Tilt ≥ 0.37, git (for running/testing the live paths)
- Optional: [GoReleaser](https://goreleaser.com) v2 for release/snapshot builds

---

## Build, test, reinstall

```bash
# from the repo root
go build ./...                       # compile everything
go test ./...                        # unit tests (no Docker needed)
go vet ./...                         # static checks

# install/refresh the binary on your PATH (current local workflow)
go build -o ~/.local/bin/berth .
hash -r                              # if an open shell still can't find it

# cross-platform snapshot build (no publishing) — verifies all targets compile
goreleaser build --snapshot --clean  # outputs to dist/
goreleaser check                     # validates .goreleaser.yaml (needs a git remote)
```

There is **no separate "reinstall" step** — `go build -o ~/.local/bin/berth .`
overwrites the binary in place. The installed binary is a static copy; rebuild
after any code change. (Published installs via Homebrew / `curl|sh` handle this
for end users; local dev uses the one build command.)

### Editor / module

- Module path: `github.com/dheerajnalapat/berth` (in `go.mod`). **Change this to
  your real GitHub path before publishing** — it appears in every internal import
  and in `.goreleaser.yaml` / `install.sh`. A repo-wide find/replace of the module
  string is enough.

---

## Repository layout

```
berth/
  main.go                  entrypoint → cmd.Execute()
  cmd/                     cobra commands (thin; delegate to internal/)
    root.go                root command + global flags (--slug, --dry-run, -v)
    up.go  down.go  ls.go  view.go  proxy.go  doctor.go  init.go  open.go  logs.go
    signal_unix.go         terminate(pid) via SIGTERM   (//go:build !windows)
    signal_windows.go      terminate(pid) via os.Process.Kill (//go:build windows)
  internal/                all logic; unit-tested
    ports/                 free TCP port allocation
    gitx/                  linked-worktree detection + name
    manifest/              .berth.toml load + validate
    slug/                  sanitize + derive + resolution ladder
    compose/               read service names from base compose
    override/              generate the compose override (!reset, labels, network)
    identity/              render hostnames from route templates
    paths/                 ~/.berth dir layout (honors BERTH_HOME)
    proxy/                 Traefik lifecycle + embedded compose template
    tiltx/                 tilt up args + Tilt-UI Traefik file-provider route
    dockerx/               query berth-managed containers via labels → []Stack
    traefikapi/            read live routers from the Traefik API
    stack/                 shared Stack model
    doctor/                preflight checks
    scaffold/              `berth init` compose inspection + manifest rendering
    ui/                    `berth view` lipgloss rendering
  .goreleaser.yaml  install.sh  .github/workflows/release.yml
  docs/                    design spec, plan, onboarding, this guide
```

**Conventions:** `cmd/` is a thin cobra layer; real logic lives in `internal/`
packages that are independently testable. Tests sit next to code as
`<file>_test.go`. Keep files focused and small.

---

## Architecture in one pass

`berth up` (see `cmd/up.go`) is the spine and shows how the packages compose:

1. `gitx.Worktree` + `manifest.Load` + `slug.Resolve` → the **slug**.
2. `dockerx.SlugOwner` → collision check (refuse if the slug is claimed by a
   different path).
3. `ports.Free` → a free Tilt UI port.
4. `compose.ServiceNames` (all services) + `identity.RenderHost` (route hosts) →
   `override.Generate` → the overlay written to `paths.Overrides()`.
5. `tiltx.RenderDynamicRoute` → the Tilt-UI route written to
   `paths.TraefikDynamic()`.
6. `proxy.Ensure` → start Traefik if needed.
7. Set env + `exec` Tilt with `tiltx.UpArgs(port)`.

`ls`/`view`/`down` derive everything from Docker labels (`dockerx`) and the
Traefik API (`traefikapi`) — **berth keeps no state file**. The only runtime
state is a detached-run pidfile under `paths.Run()`.

### The integration contract (critical, learned from live testing)

berth's only hooks into a project are **environment variables** and a **generated
compose override**. The project's Tiltfile must cooperate:

- **`tilt up --host 0.0.0.0 --port <free> -- --docker`** is what berth invokes.
  Tilt flags (`--host`, `--port`) must precede `--`; everything after `--` goes to
  the Tiltfile's `config.parse()`. The Tiltfile must `config.define_bool("docker")`.
- **`--host 0.0.0.0`** is required so the Tilt UI is reachable from the Traefik
  container via `host.docker.internal` (Tilt binds `localhost` otherwise → 502).
- **Tilt ignores `COMPOSE_PROJECT_NAME`.** The Tiltfile shim must pass
  `docker_compose(..., project_name=os.getenv("BERTH_SLUG"))`, or isolation and
  `berth down` key off the directory name instead of the slug.

These three were real bugs found during live acceptance; if you change `tiltx` or
the documented shim, preserve all three.

### The `!reset` override

`internal/override` builds the overlay as a `map[string]any` and marshals it with
`gopkg.in/yaml.v3`. Compose's `!reset` tag (strip host ports / container names) is
emitted via a custom `resetNode` type implementing `yaml.Marshaler` that returns a
`*yaml.Node` with `Tag: "!reset"`. yaml.v3 emits `ports: !reset []` and
`container_name: !reset null` correctly; the test asserts the exact substrings. If
a future yaml.v3 changes tag emission, the fallback is a sentinel value +
`bytes.ReplaceAll`.

### Proxy

`internal/proxy` embeds `traefik-compose.yml.tmpl` (kept **inside** the package —
`//go:embed` forbids parent paths) and renders it with the network name + dynamic
dir. Traefik runs as container `berth-proxy` on the external `berth` network, owns
`:80`, exposes its API/dashboard on `127.0.0.1:8080`, mounts the Docker socket
(docker provider) and the dynamic dir (file provider), and is launched with
`host.docker.internal:host-gateway` so file-provider routes can reach host
processes (the Tilt UI).

---

## Testing

- **Unit tests** (`go test ./...`) are pure and need no Docker. `gitx` shells out
  to real `git` in a temp repo; `traefikapi` uses `httptest`.
- **Live/integration** is manual (needs Docker + Tilt). The fastest end-to-end
  smoke test is a synthetic two-stack run that doesn't depend on any real project:

  ```bash
  # two trivial stacks (traefik/whoami) that both hardcode container_name and
  # publish the same host port — if both come up, berth's port-strip works.
  mkdir -p /tmp/berth-smoke/{a,b}
  for d in a b; do
    cat > /tmp/berth-smoke/$d/docker-compose.yml <<'EOF'
  services:
    web:
      image: traefik/whoami
      container_name: demo-web
      ports: ["8099:80"]
  EOF
    cat > /tmp/berth-smoke/$d/Tiltfile <<'EOF'
  config.define_bool("docker"); config.parse()
  files = ["./docker-compose.yml"]
  ov = os.getenv("BERTH_COMPOSE_OVERRIDE", "")
  if ov: files.append(ov)
  s = os.getenv("BERTH_SLUG", "")
  docker_compose(files, project_name=s) if s else docker_compose(files)
  EOF
  done
  printf 'name="demoa"\ncompose_file="docker-compose.yml"\n[[routes]]\nservice="web"\nport=80\n' > /tmp/berth-smoke/a/.berth.toml
  printf 'name="demob"\ncompose_file="docker-compose.yml"\n[[routes]]\nservice="web"\nport=80\n' > /tmp/berth-smoke/b/.berth.toml

  berth proxy up
  (cd /tmp/berth-smoke/a && berth up -d)
  (cd /tmp/berth-smoke/b && berth up -d)
  curl -s http://demoa.localhost/ | grep Host:    # → Host: demoa.localhost
  curl -s http://demob.localhost/ | grep Host:    # → Host: demob.localhost (different backend)
  berth view
  (cd /tmp/berth-smoke/a && berth down); (cd /tmp/berth-smoke/b && berth down)
  berth proxy down; rm -rf /tmp/berth-smoke
  ```

- Run `berth up --dry-run` in any onboarded project to inspect the generated
  override and the exact Tilt command/env without starting anything.

---

## Release / distribution

`.goreleaser.yaml` builds static binaries for linux/darwin/windows × amd64/arm64
and publishes to GitHub Releases + a Homebrew tap; `install.sh` is the `curl|sh`
fallback; `.github/workflows/release.yml` runs GoReleaser on a `v*` tag.

Before the first release: set the real module path / repo owner (see above), add a
git remote, and create the `homebrew-<tap>` repo referenced in `.goreleaser.yaml`.

`CGO_ENABLED=0` keeps binaries fully static. Note `syscall.Kill` is Unix-only —
process termination is split across `cmd/signal_unix.go` / `cmd/signal_windows.go`
behind build tags; keep that split if you touch detached-process handling.

---

## Roadmap / deferred (post-v1)

- **Per-slug image-tag isolation** — the Tiltfile `tag` hook exists but is disabled
  by default; enabling needs per-slug `image:` tags in compose so a rebuild in one
  worktree can't clobber another's image.
- **HTTPS** for `*.localhost` (mkcert / Traefik local CA).
- **Thin Tilt extension** — collapse the per-project Tiltfile shim to a one-liner
  (`load('ext://berth', 'berth_compose'); berth_compose()`), removing most of the
  onboarding boilerplate.
- **Non-Vite frontend presets** and **general third-party project support**.
