---
name: lane
description: Use when running or testing multiple services, or the same project across several git worktrees, in parallel — lane spins up isolated, port-conflict-free stacks each reachable at a friendly *.localhost URL.
---

# Testing in parallel with lane

`lane` brings a project's Docker/Tilt stack up behind a shared proxy, so **each
git worktree gets its own isolated stack with no host-port conflicts**. Multiple
agents (one per worktree) can bring stacks up and test concurrently.

## The loop (per worktree)

1. Bring the stack up and wait until it's serving, getting machine-readable URLs:
   ```bash
   lane up --wait --json
   ```
   Parse stdout JSON: `{"slug":"...","urls":[{"url":"http://<slug>.localhost"}], ...}`.
   (`--json`/`--wait` imply detached; human logs go to stderr; exit code 0 = ready.)
2. Run tests / requests against the returned `url`(s).
3. Tear down when done:
   ```bash
   lane down
   ```

## Test only what changed (faster, lighter)

Bring up a subset instead of the whole stack — name the services you changed
(their dependencies come up automatically):

```bash
lane up api --wait --json     # only api (+ deps)
```

Every HTTP service is auto-routed at `http://<slug>-<service>.localhost` (e.g.
`http://<slug>-api.localhost`), and the JSON `urls[]` carry a per-service
`"running"` flag; `--wait` waits only on services you started.

To save resources across many worktrees, run the changed services fresh and
**borrow the rest from a running base stack** of the same project (compose
runner):

```bash
lane up api --base --wait --json   # api fresh; db/auth/web borrowed from the base
```

`--json` then also reports `"base"`, `"fresh"`, and `"borrowed"`. Name a
dependency too (`lane up api db --base`) to run it fresh instead of borrowing it.

## Notes

- `lane ls --json` lists running stacks. Exit code `0` = success (including
  "already running"); `1` = error.
- The slug derives from the git worktree, so two worktrees of one repo get
  distinct URLs automatically — no port coordination needed.
- Use `-C <dir>` / `--path <dir>` to act on a project without `cd`-ing into it.
- First time only: `lane doctor` checks the environment; `lane proxy up` starts
  the shared proxy (lane does this automatically on `up`). Parallel `lane up`s
  are race-safe.
