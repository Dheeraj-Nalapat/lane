# Onboarding ReMind to lane (reference)

This documents the steps to onboard ReMind and run the two-worktree acceptance
test. The `.lane.toml` manifest is already created in the ReMind repo
(uncommitted, for review). The remaining step touches ReMind's `Tiltfile` —
follow ReMind's own BE_TASKS.md tracking convention when you apply it.

> Status: **lane core verified live** against a synthetic two-stack setup
> (2026-06-09) — two stacks reachable simultaneously at distinct
> `*.localhost` URLs with zero host-port conflict, Tilt UIs routed, clean
> teardown. The full ReMind run is **blocked by a pre-existing ReMind bug**
> (its `agent-server` Docker image fails to build — see "Known blocker").

## Tiltfile contract (learned from live testing)

A project's Tiltfile, in its docker branch, must:
1. **Accept `--docker`** — `config.define_bool("docker")` + `config.parse()`
   (lane always invokes `tilt up --host 0.0.0.0 --port N -- --docker`).
2. **Set `project_name` to `$LANE_SLUG`** on `docker_compose(...)` — Tilt does
   not honor `COMPOSE_PROJECT_NAME`, so without this all stacks (and
   `lane down`) key off the directory name instead of the slug.

## Known blocker (ReMind, not lane)

ReMind's `--docker` build currently fails:
```
agent-server │ cannot normalize a relative path beyond the base directory:
agent-server │   /app/../../packages/contracts
agent-server │ ERROR IN: [stage-0 7/8] RUN uv pip install --system --no-cache /packages/contracts/ .
```
This is independent of lane (same failure under a plain `tilt up -- --docker`)
and blocks the whole stack via `depends_on`. Fix `Dockerfile.agent-server`'s
`packages/contracts` path before running the full ReMind acceptance below.

## 1. Manifest (done — review it)

`ReMind/.lane.toml`:
```toml
name = "remind"
compose_file = "infra/docker-compose.yml"

[[routes]]
service = "ui"
port = 80
```

## 2. Make ReMind's Tiltfile lane-aware (gated; inert for normal `tilt up`)

In `ReMind/Tiltfile`, inside the `if use_docker:` branch, replace the single
`docker_compose("./infra/docker-compose.yml")` line with the shim below. The
`tag` variable is the per-slug image-tag hook — left empty by default in v1
(see "Deferred" in the design spec), so the `docker_build` refs are unchanged
unless `LANE_SLUG` is set and you opt in.

```python
if use_docker:
    # --- lane integration (active only under `lane up`) ---
    lane_slug = os.getenv("LANE_SLUG", "")
    tag = ""  # v1: per-slug image tags disabled. To enable: tag = (":" + lane_slug) if lane_slug else ""

    compose_files = ["./infra/docker-compose.yml"]
    lane_override = os.getenv("LANE_COMPOSE_OVERRIDE", "")
    if lane_override:
        compose_files.append(lane_override)
    # Tilt does NOT honor COMPOSE_PROJECT_NAME; pass the slug as project_name so
    # each stack (and `lane down`) is isolated by slug, not the directory name.
    if lane_slug:
        docker_compose(compose_files, project_name=lane_slug)
    else:
        docker_compose(compose_files)

    docker_build(
        "remind/platform-server" + tag,
        context=".",
        dockerfile="infra/docker/Dockerfile.server",
        live_update=[
            sync("./services/platform/src", "/app/src"),
            run("uv pip install -e .", trigger=["./services/platform/pyproject.toml"]),
            restart_container(),
        ],
    )

    docker_build(
        "remind/agent-server" + tag,
        context=".",
        dockerfile="infra/docker/Dockerfile.agent-server",
        live_update=[
            sync("./services/agent/src", "/app/src"),
            run("uv pip install -e .", trigger=["./services/agent/pyproject.toml"]),
            restart_container(),
        ],
    )

    dc_resource("server", labels=["backend"])
    dc_resource("agent-server", labels=["backend"])
    dc_resource("worker", labels=["backend"])
    dc_resource("ui", labels=["frontend"])
```

Why this is non-invasive: without `LANE_COMPOSE_OVERRIDE`/`LANE_SLUG` set
(i.e. a normal `tilt up -- --docker`), `compose_files` is just the original
single file and `tag` is empty — identical behavior to before.

## 3. Live acceptance test — two worktrees at once

Run this when `:80`/`:8080` are free and you don't mind starting containers:

```bash
# Build + install lane
cd /home/dheerajnalapat/project/lane
go build -o /usr/local/bin/lane .   # or: go install .
lane doctor                          # expect all green

# Proxy + main ReMind
lane proxy up
cd /home/dheerajnalapat/project/ReMind
lane up -d
lane ls            # shows remind → http://remind.localhost
# visit http://remind.localhost and http://tilt-remind.localhost

# Second worktree, simultaneously
cd /home/dheerajnalapat/project/ReMind
git worktree add ../remind-featx -b featx
cd ../remind-featx
lane up -d
lane ls            # shows BOTH remind and remind-featx
lane view          # live routing for both
# visit http://remind.localhost AND http://remind-featx.localhost — both load

# Teardown + prove non-invasiveness
cd ../remind-featx && lane down
cd ../ReMind && lane down
lane ls            # empty
git -C /home/dheerajnalapat/project/ReMind status --short   # clean working tree
```

Expected: both stacks reachable in the browser at the same time, zero host-port
conflicts, and after teardown the ReMind working tree is clean (all lane
wiring lived in `~/.lane/`, never in the repo).
