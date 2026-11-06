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

6. **Native command invocation directories** (2026-11-06, Tianwu D127)
   The control plane and shell already pass the per-session working directory
   into Execution.Dir. Scanner commands instead resolved file flags against
   their shared startup Base.WorkDir, so a shell-created case/raw directory
   existed while gogo wrote relative to the host root. Base.ResolvePaths now
   uses execution directory, then invocation context, then startup default;
   gogo, spray, zombie, neutron, proton, scan and katana use that boundary.
   Existing path normalizers in gogo/katana are removed in favor of the common
   resolver. No process-wide chdir or mutation of the shared command is used.
   Explicit native file arguments follow the command's shell directory,
   including after cd. This does not rewrite scanner-internal default config
   or scratch paths, and does not create missing output parent directories.
   A foreground proton with empty stdin now reports missing input rather than
   a successful no-findings scan; explicit empty files remain valid input.
   Tests: actual gogo through shell and registry, loopback-only input/output,
   shell cd, resolution precedence and concurrent calls; existing command and
   terminal regressions. The gogo directory regression fails on the previous
   binding and passes on the new one. Rollback: restore the previous checkout
   and rebuild the executor; no user data migration is performed.

7. **Null device arguments and environment contracts** (2026-11-06, Tianwu D129)
   Touch passed `/dev/null` to the built-in curl output flag on Windows.
   The embedded shell already maps null redirection correctly; curl instead
   joined that path to the case directory. Its file resolver now maps the
   POSIX null device to os.DevNull for body, header, trace and file inputs.
   Loopback HTTP regressions cover body/header/trace discard with write-out,
   and the terminal regression proves shell redirection needs no new handler.
   Native bash descriptions identify the host OS/architecture and POSIX
   interpreter; file schemas identify session-relative paths. Arsenal knowledge
   distinguishes host artifacts, the file-service root and WSL environments.
   No distribution name, forced installation or attack workflow is introduced.
   Tests: tools/curl, tools/terminal, tools/files, tools/arsenal, exts/skills.
   Rollback: restore the prior fork revision and rebuild the executor; no data
   migration or system configuration change is involved.

8. **Externally scheduled AOP mailboxes** (2026-11-06, Tianwu D129)
   Idle native sessions can start automatic model turns on peer/background
   input. A control plane owns tracked turn admission and pause, so its
   existing workspace extension now forwards the runtime's SingleTask option.
   This is opt-in; standalone sessions retain native automatic behavior.
   SingleTask already disables mailbox-triggered scheduling, while explicit
   RunTurn continuation and active mailbox admission remain available. Preserve
   the policy through native clear/compact/resume session rotation as well.
   No tools are removed and no model, strategy or fee policy is introduced.
   Tests: AOP extension -> runtime -> idle delivery -> explicit execution,
   repeated manual turns, rotation, active delivery, and the existing native
   automatic-session tests. Tianwu observes new directed commander messages
   and wakes its own paused-aware scheduler; historical replay never does.
   Rollback requires rebuilding both executables from matching revisions;
   there is no user-data migration.

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

Replacing the binary while a host is running requires stopping `tw` first.
Use Tianwu build scripts to select platform tags: Windows deliberately omits
netgo to preserve the system resolver (D125). Old binary retention or removal
is an operator choice; this workspace user requested removal.

## Rule

Do not reach for an upstream patch before proving our own usage, data, or
contract is correct. When an upstream change is justified, record the root
cause, the test, and the rollback path in the commit and in tianwu's
`agent.md`.
