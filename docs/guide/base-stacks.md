---
description: Borrow services from a running base stack with lane up --base — run only the services you changed fresh in a worktree and reuse the rest, instead of every worktree booting the whole app.
---

# Borrowing from a base stack

When only a few services changed, run just those fresh in a worktree and borrow
everything else from a running **base** stack — instead of every worktree booting
the whole app.

## Set up the base

From your main checkout, bring the full stack up normally:

    lane up            # slug = your project name, e.g. "webapp"

## Borrow from it in a worktree

In a worktree, name the services you changed and add `--base`:

    lane up api --base          # api fresh; db, auth, web borrowed from webapp
    lane up web api --base      # web + api fresh; the rest borrowed

lane finds the base by project name (the `name` in `.lane.toml`), runs your named
services without their dependencies, and wires the rest to the base's containers.
Your fresh services are reachable as usual (see *Selecting & reaching services*),
e.g. `http://webapp-featx-api.localhost`.

## Rules and tips

- A service you **name** runs fresh; everything else is **borrowed**. To run a
  dependency fresh too (e.g. a schema change), name it: `lane up api db --base`.
- Borrowed services are the base's real containers — a borrowed `db` is shared
  data. Name it to get a private one.
- `lane up ... --base --json` reports `base`, `fresh`, and `borrowed`.
- `lane down` in the worktree disconnects the borrowed containers; the base keeps
  running.

## Limitations (v1)

- Compose runner only (Tilt errors clearly).
- Same project only (a worktree borrows from another stack of the same project).
- A freshly-started service may briefly start before its borrowed dependency is
  resolvable; apps that retry their dependency connections handle this cleanly.
- Assumes the project uses compose's default network.
