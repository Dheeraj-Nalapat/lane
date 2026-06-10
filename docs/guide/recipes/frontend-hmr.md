---
description: Run a frontend dev server with hot-reload (Vite, Next.js) behind the lane proxy — bind 0.0.0.0, allow *.localhost hosts, and pass HMR WebSockets through Traefik on port 80.
---

# Recipe: a dev server with hot-reload (Vite / Next.js)

To run a frontend **dev server** (with HMR) behind the lane proxy, two things
must hold:

1. The dev server binds `0.0.0.0` (so the container is reachable).
2. If the dev server checks the `Host` header, it must allow `*.localhost`.

HMR rides the same `:80` through Traefik (WebSockets pass through).

## Vite

Vite blocks unknown `Host` headers and needs its HMR client pointed at the proxy
port. Gate it on the `LANE` env var so normal `npm run dev` is unchanged:

```ts
// vite.config.ts
const lane = !!process.env.LANE
export default defineConfig({
  server: {
    host: lane ? '0.0.0.0' : 'localhost',
    allowedHosts: lane ? ['.localhost'] : undefined,
    hmr: lane ? { clientPort: 80 } : undefined,
    proxy: {
      '/api': {
        target: process.env.LANE_API_TARGET || 'http://localhost:8000',
        changeOrigin: true,
      },
    },
  },
})
```

lane sets `LANE=1` and (if `api_target` is in `.lane.toml`)
`LANE_API_TARGET=http://<service>:<port>` when it runs.

## Next.js

Next's dev server works behind lane **with no special config** — just bind the
host. Run it as your service's command and route the port:

```yaml
# docker-compose.yml (dev)
services:
  web:
    image: node:20-alpine
    working_dir: /app
    command: sh -c "npm install && npm run dev"   # package.json: "next dev -H 0.0.0.0 -p 3000"
    volumes: [".:/app"]
```

```toml
# .lane.toml
[[routes]]
service = "web"
port = 3000
```

Verified with Next 16 (Turbopack): the app served over `http://<slug>.localhost`
and the HMR websocket (`/_next/webpack-hmr`) upgraded cleanly through the proxy —
no extra config. If an older/other Next version logs a cross-origin dev warning,
add to `next.config.js`:

```js
module.exports = { allowedDevOrigins: ['*.localhost'] }
```

## Other frameworks

Apply the two rules above: bind `0.0.0.0`, and allow the `*.localhost` host if the
server host-checks. That covers most dev servers.
