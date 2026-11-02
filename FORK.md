# Fork notes (kksty/cyber-harness)

This checkout is the **fork** `kksty/cyber-harness` of
`chainreactors/cyber-harness`. Tianwu consumes the `aiscan-full` binary built
from this tree; upstream code is never edited in the tianwu repository.

## Remotes and branches

| Remote | URL | Role |
|---|---|---|
| `upstream` | `https://github.com/chainreactors/cyber-harness.git` | original project |
| `origin` | `https://github.com/kksty/cyber-harness.git` | our fork |

- `master` tracks `upstream/master` and carries **no local commits**.
- `tianwu/fixes` is the patch branch on top of `master`. Keep it linear so
  upstream updates are a rebase, not a merge chore.

## Sync upstream

```sh
git fetch upstream
git checkout master
git merge --ff-only upstream/master
git push origin master

git checkout tianwu/fixes
git rebase master
git push --force-with-lease origin tianwu/fixes
```

If the rebase conflicts, resolve it on `tianwu/fixes`; do not merge upstream
into the patch branch. After a successful rebase, rebuild and reinstall the
binary (below) and rerun the Tianwu validation.

## Local patches

1. **`fix(bash): give foreground shells EOF on stdin`**
   `tools/terminal/interpreter.go`, `tools/terminal/bash_test.go`.
   The interpreter attached an `io.Pipe` as stdin and kept the writer open
   until the command ended, so any command that reads stdin (grep/head/read)
   blocked until the tool deadline. Foreground non-interactive shells now get
   EOF; `BashExecOptions.Stdin` still wins; interactive sessions keep the PTY
   path.

2. **`fix(session): deliver continued turn input to the inbox`**
   `agent/session/session.go`.
   A `Continue` turn called `agent.Continue` without pushing its new user
   message into the mailbox. `validateContinue` then rejected the turn with
   `cannot continue from message role: assistant` whenever the previous turn
   ended on an assistant message (every steer after a cancelled run).

Both changes are covered by tests in their packages:
`go test ./tools/terminal ./agent/session`.

## Build and deploy

```sh
CGO_ENABLED=0 go build -trimpath -buildvcs=false \
  -tags "forceposix emptytemplates noembed osusergo netgo full sqlite" \
  -o /path/to/tianwu/bin/aiscan-full.exe ./cmd/aiscan
```

Keep the previous binary as `aiscan-full.exe.bak`; replacing it while a host is
running requires stopping `tw` first.

## Rule

Do not reach for an upstream patch before proving our own usage, data, or
contract is correct. When an upstream change is justified, record the root
cause, the test, and the rollback path in the commit and in tianwu's
`agent.md`.
