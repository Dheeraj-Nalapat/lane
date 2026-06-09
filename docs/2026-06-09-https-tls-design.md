# lane — HTTPS / TLS (opt-in, mkcert) — Design Spec

**Date:** 2026-06-09
**Status:** Design complete — ready for implementation planning.
**Sub-project:** B of the generic-release effort.

## Context

lane routes `http://<slug>.localhost` today. Some apps need HTTPS locally —
secure cookies, `Secure`/`SameSite=None`, HTTPS-only APIs, service workers. This
sub-project adds **opt-in** `https://<slug>.localhost` without making the core
tool heavier.

Siblings (each its own cycle): C (robustness), E (docs), F (website).

## Goal

Let a user enable trusted `https://*.localhost` with one command, served by the
shared proxy alongside plain HTTP, with **no hard dependency** added to lane.

## Decisions

| Item | Decision |
|---|---|
| Cert source | **mkcert only** (locally-trusted CA + wildcard cert) |
| mkcert required? | **No** — lane installs/runs (HTTP) without it; mkcert needed only to enable HTTPS |
| No-mkcert path | `lane tls enable` prints install instructions and exits non-zero (no self-signed fallback) |
| HTTP when TLS on | **Keep both** http (`:80`) and https (`:443`); no redirect |
| Scope | **Global** switch (wildcard cert covers all stacks); no per-stack gating |
| On/off signal | Presence of `~/.lane/traefik/certs/cert.pem` |
| Diagnostics | In `lane tls status`, **not** `doctor` (HTTPS is optional; doctor must not fail without mkcert) |

## Non-goals

- HTTP→HTTPS redirect; per-stack TLS; ACME / public certs; removing the mkcert CA
  on `disable` (system-wide — left to the user via `mkcert -uninstall`).

## Components

### `lane tls` command

- **`enable`** — if `mkcert` not on `PATH`: print install guidance
  (`brew install mkcert` / `https://github.com/FiloSottile/mkcert`) and exit
  non-zero. Else: `mkcert -install` (CA → trust store), generate the wildcard
  cert into `~/.lane/traefik/certs/{cert.pem,key.pem}`, write
  `~/.lane/traefik/dynamic/tls.yml`, then `proxy.Up()` to (re)bind `:443`.
  Idempotent.
- **`disable`** — remove `certs/` + `tls.yml`, `proxy.Up()` to drop `:443`.
  Leaves the CA installed (documented).
- **`status`** — mkcert installed? CA installed (`mkcert -CAROOT` present)?
  wildcard cert present? proxy serving `:443`?

### Paths

Add `paths.TraefikCerts()` → `~/.lane/traefik/certs` (created by `paths.Ensure`).

### State signal

`~/.lane/traefik/certs/cert.pem` exists ⇒ TLS enabled. `proxy.Up` and `cmd/up.go`
both check it; no separate state file.

## Proxy changes (`internal/proxy`)

`renderCompose` takes a `tls bool` (proxy.Up derives it from cert presence):

- **TLS on:** add `--entryPoints.websecure.address=:443`, publish `"443:443"`,
  mount `~/.lane/traefik/certs` → `/certs:ro`.
- **TLS off:** identical to today (`:80` only).

The default certificate is supplied by the **file provider** (already watching
`/dynamic`) via a generated `tls.yml`:
```yaml
tls:
  stores:
    default:
      defaultCertificate:
        certFile: /certs/cert.pem
        keyFile: /certs/key.pem
```
`renderTLSConfig()` produces this; `lane tls enable` writes it.

Restart semantics: `proxy.Up` rewrites the compose file (now TLS-rendered) and
runs `docker compose -p lane-proxy up -d`, which recreates the container with the
new ports/mounts.

## Override changes (`internal/override`)

`Spec` gains `TLS bool`. When true, for each routed service emit a **second
router** (HTTP router unchanged):
```
traefik.http.routers.<slug>-<svc>-tls.rule=Host(`<host>`)
traefik.http.routers.<slug>-<svc>-tls.entrypoints=websecure
traefik.http.routers.<slug>-<svc>-tls.tls=true
traefik.http.routers.<slug>-<svc>-tls.service=<slug>-<svc>
```
It reuses the existing `<slug>-<svc>` service, so HTTP and HTTPS share one
backend; the default wildcard cert serves TLS. When `TLS` is false, output is
byte-identical to today.

## up + tilt-route wiring (`cmd/up.go`, `internal/tiltx`)

- `cmd/up.go`: `tls := certExists()` (cert file present) → pass to
  `override.Spec.TLS` and to the Tilt-UI route render.
- `tiltx.RenderDynamicRoute(slug, port, tls)`: when `tls`, add a `websecure` +
  `tls:true` router for `tilt-<slug>.localhost` alongside the web router.
- **Re-up note:** stacks up before `lane tls enable` keep only their HTTP router
  until re-upped. `view` reads live Traefik routers, so it reflects reality.

## Error handling

- `lane tls enable` without mkcert → install guidance + non-zero exit.
- `mkcert -install` / cert generation failure → surface stderr, non-zero exit,
  leave no half-written cert (`enable` writes to a temp then renames, or checks
  both files exist before writing `tls.yml`).

## Testing

Pure unit tests:
- `proxy.renderCompose(tls=true)` contains `:443`, `websecure`, and the certs
  mount; `(tls=false)` contains none of them.
- `renderTLSConfig()` contains `defaultCertificate` + `/certs/cert.pem`.
- `override.Generate{TLS:true}` emits `<slug>-<svc>-tls` + `entrypoints=websecure`
  + `tls=true`; `{TLS:false}` emits no `-tls` router.
- `tiltx.RenderDynamicRoute(..., true)` adds a `websecure`/`tls` router;
  `(..., false)` matches today.
- `mkcertCertNames()` returns `["*.localhost","*.*.localhost","localhost"]`.

Live smoke (manual, needs Docker + mkcert):
- `lane tls enable` → `curl https://<slug>.localhost` → 200 with a trusted cert;
  `http://<slug>.localhost` still 200; `lane tls disable` drops `:443`, http
  still 200.

## Backward compatibility

TLS off (the default, no certs) ⇒ zero behavior change: proxy `:80` only,
override output identical, no mkcert needed. HTTPS is purely additive and opt-in.
