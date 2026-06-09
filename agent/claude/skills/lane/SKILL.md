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

## Notes

- `lane ls --json` lists running stacks. Exit code `0` = success (including
  "already running"); `1` = error.
- The slug derives from the git worktree, so two worktrees of one repo get
  distinct URLs automatically — no port coordination needed.
- First time only: `lane doctor` checks the environment; `lane proxy up` starts
  the shared proxy (lane does this automatically on `up`). Parallel `lane up`s
  are race-safe.
