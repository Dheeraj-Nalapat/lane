---
description: Run a plain docker-compose project with lane — no Tiltfile or shim required. Write a .lane.toml manifest, map your web service and port, and get a *.localhost URL.
---

# Recipe: a plain docker-compose project

If your project has only a `docker-compose.yml` (no Tiltfile), lane drives
Compose directly — **no Tiltfile and no shim required.** This is the most common
case.

## 1. Manifest

```bash
lane init
```

→ `.lane.toml`:

```toml
name = "myapp"
compose_file = "docker-compose.yml"

[[routes]]
service = "web"   # the service that serves your app
port = 3000       # its container port
```

(Add more `[[routes]]` blocks for extra web entrypoints; `host = "api.{slug}"`
gives `api.<slug>.localhost`.)

## 2. Up / down

```bash
lane up          # compose runner, detached; prints the URL
lane up --build  # force a rebuild of built services
lane down
```

`lane up` is detached for compose projects (`-d` is a no-op). Re-running `lane up`
on a running stack no-ops; use `lane restart` to recreate.

## Notes

- **Built-image isolation is automatic.** For services with a `build:` section,
  lane lets Compose name the image per project (slug), so two worktrees never
  clobber each other's image.
- **No host ports are published** — lane strips them and routes via the proxy,
  which is why parallel stacks don't conflict.
- For machine use (agents/CI): `lane up --wait --json` waits until the route
  serves and prints `{slug, urls[]}`.
