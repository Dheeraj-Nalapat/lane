---
description: Bring up only part of a lane stack — pass service names or Docker Compose profiles, let dependencies and routes resolve automatically, and reach each service at <slug>-<service>.localhost.
---

# Selecting & reaching services

By default `lane up` brings up your whole stack. You can bring up just part of it.

## Bring up a subset

Pass service names (dependencies come up automatically):

    lane up api            # api + whatever it depends_on
    lane up web api

Or activate a Docker Compose profile (defined in your compose file):

    lane up --profile minimal
    lane up api --profile debug

`lane up` with no arguments still brings up everything.

## Reaching each service

Every HTTP service is reachable at a dashed host derived from the slug:

    <slug>-<service>.localhost

So in the `webapp` stack, `api` is at `http://webapp-api.localhost` and `admin`
at `http://webapp-admin.localhost` — no configuration needed. An explicit
`[[routes]]` entry overrides the host for a service (for example, the bare
`webapp.localhost`).

A service is auto-routed when lane can find a single container port for it (from
`expose:` or the container side of `ports:`). Services with no port, or more than
one, are skipped — add a `[[routes]]` entry to route those explicitly.

Disable or trim auto-routing in `.lane.toml`:

    [autoroute]
    enabled = true            # default
    exclude = ["worker"]      # never auto-route these

## For agents

`lane up --wait --json` returns the per-service URLs and whether each is running,
and waits only on the services you actually started:

    lane up api --wait --json

## Choosing the project directory

These commands act on the current directory by default. Use `-C` / `--path` to
point at another project without `cd`-ing:

    lane up api -C ../webapp

## Tilt note

With the compose runner, profiles and service selection work directly. Under the
Tilt runner, selected service names map to Tilt resources; compose profiles are
passed via the `COMPOSE_PROFILES` environment variable, which your Tiltfile shim
forwards to `docker_compose(..., profiles=...)`. If your Tiltfile does not forward
profiles, use service-name selection (works on both runners) or the compose runner.
