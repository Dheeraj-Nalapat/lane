# Recipe: a Tilt project

If your project uses [Tilt](https://tilt.dev), lane drives it (keeping
live-reload + the Tilt dashboard). lane auto-detects the Tiltfile and uses the
Tilt runner. You add a small, **gated** shim to your Tiltfile's docker branch.

## 1. Manifest

```toml
name = "myapp"
compose_file = "infra/docker-compose.yml"

[[routes]]
service = "ui"
port = 80
```

## 2. Tiltfile shim (in the `--docker` branch)

lane's only hook is environment variables, so the Tiltfile must read them. In the
Docker-mode branch:

```python
config.define_bool("docker")
cfg = config.parse()
use_docker = cfg.get("docker", False)

if use_docker:
    lane_slug = os.getenv("LANE_SLUG", "")
    tag = (":" + lane_slug) if lane_slug else ""   # per-slug image isolation

    compose_files = ["./infra/docker-compose.yml"]
    override = os.getenv("LANE_COMPOSE_OVERRIDE", "")
    if override:
        compose_files.append(override)

    # Tilt does NOT honor COMPOSE_PROJECT_NAME — pass the slug as project_name.
    if lane_slug:
        docker_compose(compose_files, project_name=lane_slug)
    else:
        docker_compose(compose_files)

    docker_build("myapp/server" + tag, context=".", dockerfile="infra/Dockerfile", live_update=[...])
    # dc_resource(...) as before
```

This is inert without lane: a plain `tilt up -- --docker` (no `LANE_*` env) falls
back to the original single `docker_compose(...)`.

## 3. Up / down

```bash
lane up           # foreground (Tilt logs); -d to detach
lane down
```

The Tilt UI is routed at `http://tilt-<slug>.localhost`.

## Worked example

See [`docs/onboarding-remind.md`](../../onboarding-remind.md) — the ReMind
project onboarded end-to-end (two worktrees running at once).
