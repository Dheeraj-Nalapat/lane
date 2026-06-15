## Running & testing with lane

This project uses [lane](https://github.com/Dheeraj-Nalapat/lane) to run its
Docker/Tilt stack behind a shared proxy, so each git worktree gets an isolated,
port-conflict-free stack. Multiple agents (one per worktree) can bring up stacks
and test in parallel without colliding on host ports.

The loop:

- `lane up --wait --json` — bring up an isolated stack for this worktree, wait
  until it serves, and print `{"slug":...,"urls":[{"url":"http://<slug>.localhost"}]}`
  on stdout (human logs go to stderr; exit code `0` = ready).
- Run tests / requests against the returned `url`(s).
- `lane down` — tear the stack down (the repo is left byte-for-byte unchanged).

Test only what changed (faster, lighter):

- `lane up <service...> --wait --json` — bring up only the services you changed
  (their dependencies come up automatically). Each is auto-routed at
  `http://<slug>-<service>.localhost`; the JSON `urls[]` carry a per-service
  `running` flag.
- `lane up <service> --base --wait --json` — run the changed service fresh and
  borrow the rest from a running base stack of the same project (saves resources).

Notes:

- `lane ls --json` lists running stacks. Exit `0` = success (including already
  running), `1` = error.
- `-C <dir>` / `--path <dir>` acts on a project without `cd`-ing into it.
- The slug derives from the git worktree, so two worktrees of one repo get
  distinct URLs automatically — no port coordination needed.
