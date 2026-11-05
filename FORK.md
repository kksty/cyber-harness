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

3. **`feat(usage): report the resolved context_window with token usage`**
   `agent/retry.go`, `agent/loop.go`, `agent/loop_test.go`.
   The model window is resolved inside the agent (`ModelContextWindow`) but a
   control plane only saw `context_tokens`, so Tianwu duplicated the model table
   to place its 85% handoff line (D80) — wrong for any model whose real window
   differs from the profile fallback. `usage.detail.context_window` now carries
   the resolved window, and the transcript keeps it scalar instead of summing it
   with the additive detail counters.

4. **`feat(session): per-session working directory`**
   `agent/session/protocol.go`, `agent/session/session.go`, `agent/session/workdir_test.go`,
   `agent/types.go`, `agent/loop.go`, `agent/workdir_test.go`, `tools/files/*`,
   `tools/playwright/*`, `core/tool` (no change needed; tools already read
   `operation.WorkDirFromContext`).
   Upstream AOP has no session working directory: `OpenSessionRequest` has no
   work-dir field and one agent process serves every run, so tools resolve
   relative paths against the agent's process directory. Tianwu sends a work-dir
   extension on `OpenSessionRequest` (`type.googleapis.com/tianwu.WorkDir`,
   JSON `{"work_dir":"<absolute path>"}`); the agent stores it per session
   (`SessionOptions.WorkDir` → `agent.Config.WorkDir`), puts it in each tool
   invocation's `operation.Invocation.WorkDir`, and the files/playwright tools
   resolve their relative input/output paths against it. The `os.Root` boundary
   stays the agent's configured directory: a workspace outside it is refused.
   Empty/absent extension keeps the old behavior, so old control planes and new
   agents stay compatible. Tests: `go test ./agent ./agent/session ./tools/files ./tools/playwright`.

5. **`fix(provider): activate a slow but working endpoint`**
   `agent/provider/state.go`, `agent/provider/state_test.go`.
   The probe that decides whether a configured endpoint becomes this node's
   provider used a hard-coded 5s timeout. A gateway whose first token takes
   3-7s is usable but failed that probe; the caller treated the reload as
   rejected, discarded the candidate, and kept the provider-disabled profile —
   every session then failed with `agent provider is nil`, and the control
   plane's readiness gate (agent present but no provider/model) never passed.
   Activation (`Reload`) now uses the full request probe budget
   (`llmProbeTimeout`, 30s); the fast 5s probe remains for live `Update`
   replacement, where failing keeps the previous client. The probe helper takes
   an explicit timeout so the two paths cannot drift apart again.
   Test: `go test ./agent/provider -run TestReloadKeepsAProviderWhoseEndpointIsSlowToAnswer`.

All changes are covered by tests in their packages:
`go test ./tools/terminal ./tools/playwright ./tools/files ./agent/session ./agent`.

## Build and deploy

Keep the checkout on `tianwu/fixes` (not `master`) while building: `master`
has no local patches.

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
