---
description: Get started with lane in 10 minutes — onboard a docker-compose or Tilt project, bring up an isolated dev stack at a *.localhost URL, and run multiple git worktrees in parallel with no port conflicts.
---

# Getting started with lane

lane runs your project's stack behind a shared proxy so it's reachable at a
friendly `*.localhost` URL — and so **every git worktree gets its own isolated
stack with no port conflicts**. This is the first 10 minutes.

## Prerequisites

- Docker (≥ 28) with Compose ≥ 2.20
- git
- Tilt — only if your project uses it (plain docker-compose projects don't need it)

Check everything at once:

```bash
lane doctor
```

## 1. Onboard your project

From a project that has a `docker-compose.yml` (or a Tiltfile + compose):

```bash
lane init        # writes a .lane.toml (inspects your compose to guess the web service)
```

Review the generated `.lane.toml`:

```toml
name = "myapp"
compose_file = "docker-compose.yml"

[[routes]]
service = "web"
port = 3000
```

## 2. Bring it up

```bash
lane up
```

lane starts the shared proxy (first time), generates a non-invasive compose
overlay, and brings your stack up. Open the printed URL:

```
http://myapp.localhost
```

See what's running:

```bash
lane ls        # quick table
lane view      # live, interactive control panel (q to quit)
```

## 3. A second worktree — at the same time

The payoff: run the *same* project twice without port clashes.

```bash
git worktree add ../myapp-featx -b featx
cd ../myapp-featx
lane up
```

Now both are reachable simultaneously:

```
http://myapp.localhost          # main checkout
http://myapp-featx.localhost    # the worktree
```

The slug (and URL) is derived from the git worktree, so they never collide.

## 4. Tear down

```bash
lane down            # this stack (keeps volumes)
lane down --volumes  # also wipe its named volumes
```

## Next steps

- Your project type: [plain compose](recipes/compose.md) ·
  [Tilt](recipes/tilt.md) · [frontend HMR](recipes/frontend-hmr.md)
- [HTTPS](https.md) and [using lane with agents](agents.md) (parallel testing).
