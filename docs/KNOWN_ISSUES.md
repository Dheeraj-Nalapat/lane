# Known Issues & Backlog

A running list of known limitations and rough edges in lane. Each entry is
self-contained enough to pick up as a task: symptom → cause → workaround →
proposed work. Add new issues at the bottom; move resolved ones to the
[Resolved](#resolved) section with the fixing commit/PR.

| # | Title | Area | Severity | Status |
|---|-------|------|----------|--------|
| 1 | Tilt can't share/borrow resources from a base stack | base-borrowing, tiltx | High | Open |
| 2 | Dev-server packages (Expo / Metro) don't auto-route cleanly | routing, compose | Medium | Open |

---

## 1. Tilt can't share/borrow resources from a base stack

**Area:** `internal/basex`, `internal/tiltx`, `cmd/up.go`, `cmd/down.go`
**Severity:** High &nbsp;·&nbsp; **Status:** Open

### Symptom
`lane up <services...> --base` works for Compose projects but errors out on Tilt
projects. A worktree of a Tilt project therefore can't reuse a base stack's
already-running services — it must boot every service fresh, which defeats the
point of base-borrowing for the runner most users start with.

> Documented today as a v1 limitation in `docs/guide/base-stacks.md`:
> *"Compose runner only (Tilt errors clearly)."*

### Cause
Base-borrowing is implemented entirely on the Compose path:
- A worktree brings up only its named services with `compose --no-deps`.
- lane then `docker network connect`s each borrowed base container into the
  worktree's compose network (`<slug>_default`) under the service-name alias.

Tilt manages its own resources and containers and does not expose a stable
"default network + service-name alias" model the way Compose does, so there is
no equivalent borrowing path. The code errors clearly instead of doing the
wrong thing.

### Workaround
- Use the Compose runner if you need base-borrowing.
- On Tilt, bring up the full stack per worktree (no borrowing).

### Proposed work
- [ ] Map how Tilt names/networks the containers it manages in lane's docker
      mode (inspect `lane.*` labels and the network Tilt attaches containers to).
- [ ] Decide a borrowing strategy for Tilt: either
      (a) selectively bring up named Tilt resources and `network connect` the
      base's containers into Tilt's network, or
      (b) front borrowed services purely at the Traefik/routing layer without a
      shared Docker network.
- [ ] Extend `internal/basex` so base discovery + borrowed-set computation is
      runner-agnostic; gate the connect step behind a runner capability.
- [ ] Update `docs/guide/base-stacks.md` once Tilt is supported (remove the
      "Compose runner only" limitation).

### Related
- `docs/superpowers/plans/2026-06-10-shared-base-services.md`
- Commits: `682cd34` (`--base` mode), `8dfd0c9` (basex), `945d08a` (down disconnect)

---

## 2. Dev-server packages (Expo / Metro) don't auto-route cleanly

**Area:** `internal/routing`, `internal/compose`
**Severity:** Medium &nbsp;·&nbsp; **Status:** Open

### Symptom
Frontend dev bundlers like **Expo** (and **Metro** / React Native generally) are
either skipped by auto-routing or don't work correctly behind the Traefik proxy:
HMR/live-reload websockets fail, or the bundler emits manifest URLs pointing at
the wrong host/port.

### Cause
Several things compound:
1. **Multiple ports.** Auto-routing only routes a service when it can find a
   *single* exposed container port (see `internal/routing/routing.go` and
   `discoverPort` in `internal/compose/compose.go`). Expo/Metro expose several
   ports (bundler `:8081`, dev tools, tunnel), so the service is ambiguous and
   gets skipped.
2. **Websockets + absolute URLs.** Expo/Metro serve a manifest and run HMR over
   websockets using absolute `host:port` URLs. Behind the proxy everything rides
   `:80` on `<slug>-<service>.localhost`, so hardcoded URLs and host checks
   don't line up (same class of problem the Vite recipe solves with
   `hmr.clientPort = 80`).
3. **Packager hostname.** Expo derives client URLs from
   `REACT_NATIVE_PACKAGER_HOSTNAME` / packager proxy env, which lane doesn't set,
   so clients try to reach the bundler at the container's internal address.

### Workaround
- Add an explicit `[[routes]]` entry pinning the bundler's port so it routes
  despite multiple exposed ports.
- Mirror the Vite pattern from `docs/guide/recipes/frontend-hmr.md`: bind
  `0.0.0.0`, allow `*.localhost` hosts, and point the HMR/websocket client at
  port 80.

### Proposed work
- [ ] Reproduce with a minimal Expo (and a Metro) service to capture exactly
      which URLs/ports break behind the proxy.
- [ ] Investigate the relevant env knobs (`REACT_NATIVE_PACKAGER_HOSTNAME`,
      `EXPO_PACKAGER_PROXY_URL`, `EXPO_DEVTOOLS_LISTEN_ADDRESS`) and whether lane
      should set them like it sets `LANE` / `LANE_API_TARGET`.
- [ ] Add an **Expo / Metro recipe** under `docs/guide/recipes/` documenting the
      working config.
- [ ] Consider letting auto-routing pick a *primary* port via a hint (e.g.
      `[autoroute] primary_port` or a label) so multi-port dev servers aren't
      silently skipped.

### Related
- `docs/guide/recipes/frontend-hmr.md` (Vite/Next.js precedent)
- `docs/guide/selecting-services.md` (auto-route rules: single exposed port)

---

## Resolved

_None yet._
