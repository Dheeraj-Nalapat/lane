# Onboarding ReMind to berth (reference)

This documents the steps to onboard ReMind and run the two-worktree acceptance
test. The `.berth.toml` manifest is already created in the ReMind repo
(uncommitted, for review). The remaining step touches ReMind's `Tiltfile` —
follow ReMind's own BE_TASKS.md tracking convention when you apply it.

> Status: **not yet executed live.** The acceptance run binds host `:80` and
> starts containers; it was deferred to avoid disturbing other running stacks.

## 1. Manifest (done — review it)

`ReMind/.berth.toml`:
```toml
name = "remind"
compose_file = "infra/docker-compose.yml"

[[routes]]
service = "ui"
port = 80
```

## 2. Make ReMind's Tiltfile berth-aware (gated; inert for normal `tilt up`)

In `ReMind/Tiltfile`, inside the `if use_docker:` branch, replace the single
`docker_compose("./infra/docker-compose.yml")` line with the shim below. The
`tag` variable is the per-slug image-tag hook — left empty by default in v1
(see "Deferred" in the design spec), so the `docker_build` refs are unchanged
unless `BERTH_SLUG` is set and you opt in.

```python
if use_docker:
    # --- berth integration (active only under `berth up`) ---
    berth_slug = os.getenv("BERTH_SLUG", "")
    tag = ""  # v1: per-slug image tags disabled. To enable: tag = (":" + berth_slug) if berth_slug else ""

    compose_files = ["./infra/docker-compose.yml"]
    berth_override = os.getenv("BERTH_COMPOSE_OVERRIDE", "")
    if berth_override:
        compose_files.append(berth_override)
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

Why this is non-invasive: without `BERTH_COMPOSE_OVERRIDE`/`BERTH_SLUG` set
(i.e. a normal `tilt up -- --docker`), `compose_files` is just the original
single file and `tag` is empty — identical behavior to before.

## 3. Live acceptance test — two worktrees at once

Run this when `:80`/`:8080` are free and you don't mind starting containers:

```bash
# Build + install berth
cd /home/dheerajnalapat/project/berth
go build -o /usr/local/bin/berth .   # or: go install .
berth doctor                          # expect all green

# Proxy + main ReMind
berth proxy up
cd /home/dheerajnalapat/project/ReMind
berth up -d
berth ls            # shows remind → http://remind.localhost
# visit http://remind.localhost and http://tilt-remind.localhost

# Second worktree, simultaneously
cd /home/dheerajnalapat/project/ReMind
git worktree add ../remind-featx -b featx
cd ../remind-featx
berth up -d
berth ls            # shows BOTH remind and remind-featx
berth view          # live routing for both
# visit http://remind.localhost AND http://remind-featx.localhost — both load

# Teardown + prove non-invasiveness
cd ../remind-featx && berth down
cd ../ReMind && berth down
berth ls            # empty
git -C /home/dheerajnalapat/project/ReMind status --short   # clean working tree
```

Expected: both stacks reachable in the browser at the same time, zero host-port
conflicts, and after teardown the ReMind working tree is clean (all berth
wiring lived in `~/.berth/`, never in the repo).
