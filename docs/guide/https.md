---
description: Enable trusted HTTPS for lane's *.localhost stacks with mkcert — generate a wildcard cert, serve https://<slug>.localhost for secure cookies and HTTPS-only APIs, and toggle TLS on or off.
---

# HTTPS (optional)

lane serves HTTP by default. For trusted `https://<slug>.localhost` (secure
cookies, HTTPS-only APIs), install [mkcert](https://github.com/FiloSottile/mkcert)
and enable TLS:

```bash
mkcert -install     # one-time; adds a local CA to your trust store
lane tls enable     # generates a wildcard cert, restarts the proxy on :443
lane up             # re-up running stacks to add their HTTPS route
```

Both `http://` and `https://` then serve every stack. `lane tls status` shows
state; `lane tls disable` returns to HTTP-only. mkcert is **not** required for
normal use — only to enable HTTPS. Nested hosts (`api.<slug>.localhost`) aren't
covered by the wildcard cert.
