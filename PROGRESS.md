# Pane Progress

This file is intentionally committed so every machine and every agent can see the current plan, what exists, and what should happen next.

## Current status

**V1 is complete.** Dogfooded 2026-05-21 on real default daemon/DB with multiple panes. All V1 checks passed.

Pane has a working daemon-backed core with sessions, board, summary, messaging, file activity, git guardrails, shell integration, continuity, heartbeat, agent state, and session lifecycle cleanup.

**V2 is complete.** Real dogfood found an overlap attribution gap, and the fix has been implemented and smoke-tested. See the V2 dogfood notes below.

See `ROADMAP.md` for the V2/V3/Done plan.

Validated recently with:

```bash
go test ./...
go vet ./...
make build
```

Also smoke-tested locally with temporary database/socket paths.

## What works now

### Build and test

- `make build` builds `bin/pane`
- `go test ./...` passes
- `go vet ./...` passes

### Daemon

- `pane daemon start`
- `pane daemon status`
- `pane daemon health`
- `pane daemon stop`
- Unix socket request/response protocol
- SQLite opened once by the daemon
- PID file written while daemon runs
- duplicate foreground starts become clear no-ops when daemon is healthy

### Sessions

- `pane init`
- `pane heartbeat`
- `pane status`
- `pane intent <text>`
- pane identity from Zellij/tmux/TTY
- workspace/branch/cwd detection
- session create/resume
- daemon is source of truth for session commands

### Shared board

- `pane board`
- shows active/idle sessions in the workspace
- includes session id, status, branch, cwd, intent, last seen
- includes compact coordination indicators for unread messages and awaiting replies
- includes recent files from daemon-observed activity
- daemon-backed

### Summary

- `pane summary`
- current-session startup/resume view
- shows current session and peer sessions
- surfaces unread messages and awaiting-reply counts for the current session
- includes current session recent files
- daemon-backed

### Messaging

- `pane ask <session-id> <message>`
- `pane inbox`
- `pane reply <message-id> <message>`
- queued/delivered message state
- message threads via `thread_id`
- daemon-backed

### Continuity and history

- `pane continue <session-id>`
- `pane history [--since <duration>]`
- parent-session lineage
- continuity context in `pane summary`
- daemon-backed

### Generic agent state

- `pane state set <namespace.key> <json>`
- `pane state get <namespace.key>`
- `pane state list [namespace-prefix]`
- `pane state delete <namespace.key>`
- workspace-scoped JSON state
- daemon-backed

### Shell/git integration

- `pane shell-init`
- prompt heartbeat through `pane heartbeat`
- `pane shims install`
- `pane git <args...>` real passthrough with first-pass preflight and event recording

## V1 ready target

`V1_READY.md` defines the bar. **V1 is achieved.** All V1 criteria passed the dogfood checklist on 2026-05-21.

See `ROADMAP.md` for V2, V3, and Done scoping.

## What is not real yet

These are important but not implemented yet:

- file working-set overlap detection
- daemon auto-start outside shell hook
- full daemon lock/log lifecycle beyond first-pass PID/status
- platform-native file watcher
- richer session lineage beyond first parent links
- richer `pane history` filters and summaries
- richer generic `pane state` workflows beyond first-pass key/value JSON
- richer aliases/names beyond first-pass short session IDs
- richer session lifecycle cleanup beyond first-pass close/prune/stale hiding

## Product interpretation to preserve

Pane is shared local memory for agent work over a workspace.

It is not primarily a git tool. Git preflight is one useful guardrail, not the center of the product.

It is not an orchestrator. Pane does not assign tasks or control agents from above.

It is not a dashboard. The board is a coordination surface agents read from and write to.

The key workflow assumption is that agents maintain their own state:

- agents run Pane commands themselves
- agents update `pane intent` when switching tasks
- agents inspect `pane board` and `pane summary`
- agents use `pane ask` / `pane reply` instead of routing context through the human
- agents run risky commands through guardrails such as `pane git`

The human can inspect and override, but the human should not be responsible for keeping Pane state current.

## Reframed long-term direction

`REFRAMING.md` expands the lens:

Pane is not just concurrent coordination. It can become the unified persistence and awareness layer for all agent workflows.

Three scopes:

1. **Sequential continuity** — new sessions inherit context from previous sessions.
2. **Concurrent coordination** — active sessions share board, messages, working sets, warnings.
3. **Agent memory** — specialized agents store persistent namespaced state through Pane.

This does not change V1. V1 remains the foundation.

## Phase plan

### Phase 1 — daemon-owned sessions and board

Status: complete.

Completed:

- daemon owns session store/manager
- `pane init` through daemon
- `pane status` through daemon
- `pane intent` through daemon
- `pane board` through daemon
- CLI no longer opens SQLite directly for session/board flows

### Phase 2 — session summary

Status: complete first pass.

Completed:

- `internal/summary` model and renderer
- daemon `GetSummary` handler
- `pane summary` command
- summary tests and daemon handler coverage

Still needed later:

- include message state
- include file activity
- include lineage/history
- decide whether auto-injected summaries should print to stderr

### Phase 3 — messaging

Status: complete first pass.

Completed:

- message model and ID generation
- message store
- inbox renderer
- daemon `MessageSend`, `MessageList`, `MessageReply`
- CLI `pane ask`, `pane inbox`, `pane reply`
- smoke-tested ask/inbox/reply between simulated panes

Still needed later:

- show unread/open messages in board and summary
- validate or warn on nonexistent target sessions
- improve session targeting ergonomics
- decide whether inbox should mark delivered immediately or require explicit ack

### Phase 4 — message-aware board and summary

Status: complete first pass.

Goal:

Agents should not need to remember to run `pane inbox` to know coordination state exists. Board and summary should surface message state.

Completed:

1. Added message store queries for:
   - unread count by session
   - queued messages for current session
   - open outbound threads from current session
2. Added a coordination section to `pane summary`:
   - unread messages
   - sent questions waiting for reply
3. Added compact message indicators to `pane board`:
   - unread count per session
   - awaiting reply count per session
4. Kept board output compact; full message bodies stay in summary/inbox.

Exit criteria:

- after `pane ask`, target session sees a visible unread indicator in summary/board — done
- `pane inbox` still shows full message bodies — done
- board remains compact — done

Remaining hardening:

- Refine open-thread semantics after more dogfooding.
- Decide whether summary should show full unread bodies or only previews.
- Improve message targeting ergonomics.

### Phase 5 — file activity and working sets

Status: first pass implemented.

Goal:

The board reflects observed work, not just declared intent.

Completed:

1. Implemented file activity store.
2. Added a simple polling watcher started for registered workspaces.
3. Filtered `.git/`, `.internal/`, build output, common dependency/build directories, and DB artifacts.
4. Added heuristic attribution:
   - single active session = high confidence
   - cwd prefix match = medium confidence
   - last seen fallback = low confidence
5. Derived recent files per session.
6. Show recent files in board and summary.

Still needed:

- replace or augment polling with platform-native watcher if needed
- respect `.gitignore` and `.paneignore`
- derive hot directories
- derive overlap between sessions
- tune attribution during dogfooding

### Phase 6 — git guardrail

Status: complete first pass.

Goal:

`pane git ...` behaves like real git while adding shared-state preflight warnings for risky commands.

Completed:

1. Real git passthrough preserving stdout/stderr and exit code.
2. Daemon `GitPreflight` handler.
3. Daemon `GitRecord` handler.
4. Narrow first-pass branch/session warnings.
5. Daemon-down behavior warns and continues for git.
6. Git event storage.

Still needed:

- richer preflight checks based on file activity overlap
- confirmation prompt instead of hard block for forceful operations
- PATH shim generation
- more complete target-branch parsing and remote/push semantics

### Phase 7 — shell/agent integration

Status: complete first pass.

Goal:

Agents naturally participate in Pane without humans manually driving every update.

Completed:

1. `pane shell-init` prints a shell hook.
2. Shell hook starts daemon if needed, initializes the session, prints summary, and heartbeats on prompt for bash/zsh.
3. `pane shims install` creates a transparent git shim in `~/.pane/shims`.
4. `AGENTS.md` gives agent operating instructions.

Still needed:

- dogfood the shell hook in real terminals
- consider daemon PID/log management and non-shell auto-start
- decide how aggressive heartbeat should be
- add installer ergonomics

### Phase 8 — sequential continuity

Status: first pass implemented.

Goal:

Replace manual handoffs with accumulated session lineage.

Completed:

1. Added parent-session lineage to the session model.
2. Added `pane continue <session-id>` to link the current pane/session to a previous session.
3. Added continuity context to `pane summary`, including continued-from and recent workspace history.
4. Added `pane history [--since <duration>]` for recent workspace session history.

Still needed:

- accumulate structured decisions/open threads beyond intents/messages
- improve history filtering and output shape after dogfooding
- handle long lineage chains, not only immediate parent + recent history

### Phase 9 — generic agent state

Status: first pass implemented.

Goal:

One local persistence API for specialized agents.

Completed:

1. Added workspace-scoped `agent_state` SQLite storage.
2. Added daemon-backed state handlers.
3. Added JSON-valued CLI commands:

```bash
pane state set <namespace.key> <json>
pane state get <namespace.key>
pane state list [namespace-prefix]
pane state delete <namespace.key>
```

Still needed:

- decide conventions for namespaces and ownership
- decide whether state should be workspace-only, global, or both
- add richer query/output formats if dogfooding needs them
- surface selected state in summaries when useful

### Phase 10 — heartbeat hardening

Status: first pass implemented and manually dogfooded.

Goal:

Make shell/agent participation less noisy and less dependent on re-running full session init.

Completed:

1. Added daemon-backed `pane heartbeat`.
2. Heartbeat refreshes cwd, branch, tty, last seen, and active status while preserving current intent.
3. Heartbeat creates/resumes a session if none exists for the pane/workspace, preserving shell-hook convenience.
4. Updated `pane shell-init` to use `pane heartbeat` instead of quietly re-running `pane init` on every prompt.

Manual validation:

- `pane heartbeat` preserved intent while refreshing the session.
- Real-pane continuity handoff passed after fixing intent inheritance.
- `pane state` set/get/list/delete passed across panes in the same workspace.

Still needed:

- daemon PID/lock/log lifecycle hardening
- auto-start behavior outside shell hook
- decide whether long-running agent commands need activity signals beyond prompt heartbeat

### Phase 11 — daemon lifecycle hardening

Status: first pass implemented.

Goal:

Make the daemon feel like dependable local infrastructure instead of a foreground process the user has to babysit.

Use case:

Agents should be able to rely on Pane memory being available without the human remembering which terminal is running the daemon, whether an old socket is stale, or where logs went.

Completed:

1. Added first-pass daemon PID file support.
2. Added `pane daemon status` showing running/stopped state plus pid, socket, DB, PID file, and log paths.
3. Extended daemon health payload with pid/path metadata.
4. Made duplicate `pane daemon start` calls a clear no-op when an existing daemon is healthy.
5. Smoke-tested stopped/running/status/start-again/stop with temp DB/socket/PID/log paths.

Still needed:

- process locking so two daemons do not fight over the same DB/socket in edge cases
- stronger stale PID/process detection
- actual log redirection/lifecycle beyond exposing the configured log path
- safer startup behavior around stale sockets beyond current remove-and-listen behavior
- consider `pane daemon start --background` or CLI auto-start policy beyond shell hook

Exit criteria:

- users can tell whether the daemon is running, where it is logging, and which DB/socket it owns — first pass done
- duplicate daemon starts fail clearly or become no-ops — first pass done
- stale sockets/PIDs do not confuse normal startup — still needs hardening

### Phase 12 — targeting ergonomics and overlap

Status: first pass implemented for targeting; overlap still pending.

Goal:

Make Pane easier to use during real multi-agent work by reducing long-ID friction and surfacing overlap more directly.

Completed:

1. Added short session IDs in board output.
2. Added short-ID/prefix resolution for session references.
3. `pane ask` can target a session by short ID/prefix.
4. `pane continue` can continue from a session by short ID/prefix.
5. `pane ask` now fails clearly when the target session does not exist instead of queuing to an invalid ID.

Still needed:

- derive working-set overlap from recent file activity
- surface overlap warnings in board/summary and git preflight
- richer aliases or human-friendly names if short IDs are still awkward
- dogfood whether board stays readable with short-ID indicators

### Phase 13 — session lifecycle cleanup / board freshness

Status: first pass implemented; needs real-pane dogfooding.

Goal:

Make the board reflect live work more accurately by retiring or hiding sessions that are no longer participating.

Use case:

A user may only have three panes open but see more sessions because Pane durably remembers prior pane identities. Daemon restarts do not delete sessions, and old sessions can remain active/idle without a clear close/prune/expiry path.

Completed:

1. Defined first-pass stale semantics: active/idle sessions older than 30 minutes are hidden from default active views.
2. Added `pane close` for explicitly closing the current session.
3. Added `pane sessions prune` to close stale active/idle sessions in the current workspace.
4. `pane board` now hides stale/closed sessions by default through the daemon's active-session query.
5. `pane history` still shows durable past sessions.
6. Documented that daemon restart preserves SQLite state and does not clean sessions.

Still needed:

- real-pane dogfood to confirm board count matches user expectations
- decide whether 30 minutes is the right stale threshold
- possibly add `pane board --all` or `pane sessions list` for explicit lifecycle inspection
- clearer stale/closed labels in history output

Exit criteria:

- board session count matches the user's intuitive active workspace more closely — first pass ready for dogfooding
- old sessions remain available through history but do not clutter the active board — first pass done
- agents and humans understand when Pane creates, resumes, idles, closes, or prunes sessions — docs first pass done

### V1-ready checkpoint

Status: **DONE** — dogfooded 2026-05-21 on real default daemon/DB.

All 7 required checks passed:

1. ✅ Rebuilt (`make build`) and started real daemon on default `~/.pane/` paths.
2. ✅ 3 panes initialized with distinct intents (simulated via `ZELLIJ_PANE_ID=101/102/103`).
3. ✅ `pane sessions prune` reported 0 stale; `pane board` showed exactly 3 active sessions.
4. ✅ `pane close` on one pane dropped it from board (2 remaining); `pane history --since 24h` still showed it as closed.
5. ✅ `pane ask <short-id>` delivered message; target pane received it in `pane inbox`.
6. ✅ `pane summary`, `pane state set/get`, `pane git status`, and `pane daemon status` all worked correctly.
7. ✅ V1 marked as done.

V1 is complete. V2 work can proceed. See `ROADMAP.md` for the guided plan.

### Phase 14 — overlap detection

Status: complete (V2.1).

Goal:

Surface when sessions are touching the same files/directories so agents can coordinate before conflicting work accumulates.

Completed:

1. Store query: OverlapByWorkspace returns files touched by 2+ active sessions.
2. Activity computation: ComputeOverlap derives per-pair overlaps.
3. Board overlap section with ⚠️ indicators using short IDs.
4. Summary overlap section filtered to current session's peers.
5. Git preflight file-level overlap warnings.
6. Full test coverage across store, activity, board, summary, preflight, and daemon layers.

### Phase 15 — richer git preflight

Status: complete (V2.2).

Goal:

Make git guardrails more precise and less disruptive.

Completed:

1. Confirmation prompt instead of hard block for forceful operations.
2. Better git command parsing: checkout -b/switch -c branch creation detection, push remote/branch parsing.
3. Command-specific risk warnings: rebase, merge, reset --hard, force push, checkout/switch.

### Phase 16 — board and summary signal quality

Status: complete (V2.3).

Goal:

Make board/summary output denser and more useful without becoming noisy.

Completed:

1. Hot directory derivation (directories with 2+ recently active files) per session.
2. Recent git events section on board (last 5 workspace git operations).
3. `pane board --all` flag to show all sessions including closed/stale.
4. History output with status emojis, short IDs, and shortened parent references.

### Phase 17 — daemon hardening

Status: complete (V2.4).

Goal:

Make the daemon reliable enough that users never think about whether it's running.

Completed:

1. Exclusive process lock via flock on `~/.pane/pane.lock` — prevents two daemons from racing on startup.
2. Stale PID/socket detection and cleanup — on startup, checks if previous daemon PID is alive; dead process → auto-cleans stale socket and PID file; live process → clear error message.
3. Log redirection and rotation — daemon redirects stdout/stderr to log file internally; size-based rotation (>10MB → pane.log.1, 1 backup kept).
4. `pane daemon start --background` — re-execs as detached child via setsid, parent waits up to 3s for health confirmation then exits.

New files: `internal/daemon/lock.go`, `internal/daemon/logging.go`, `internal/daemon/background.go` with 12 unit tests.

Smoke tested end-to-end: background start, dual-start race, kill -9 crash recovery, clean shutdown, self-logging.

### Phase 18 — targeting and naming ergonomics

Status: complete (V2.5).

Goal:

Make session targeting natural — names instead of opaque hex IDs.

Completed:

1. `pane name <name>` command — sets a human-friendly name for the current session.
2. Name-based resolution in `pane ask` — exact name, name prefix, case-insensitive.
3. Resolution priority: exact ID → exact name → short ID prefix → name prefix.
4. Names shown throughout: board, summary, history, overlap warnings, git events.
5. Additive migration adds `name` column to sessions table.
6. UPSERT preserves existing name when session is re-saved without one (`COALESCE(excluded.name, sessions.name)`).

Exit criteria met: `pane ask auth-refactor "are you done?"` works and reads naturally.

Smoke tested end-to-end: naming, board/summary/history rendering, ask by name, ask by prefix, overlap with names.

### Phase 19 — file watcher hardening

Status: complete (V2.6).

Goal:

Replace polling with platform-native file watching. Filter noise from build artifacts and dependencies.

Completed:

1. `NativeWatcher` using `rjeczalik/notify` — FSEvents on macOS, inotify on Linux, recursive watching.
2. `IgnoreFilter` — loads `.gitignore` and `.paneignore` from workspace root, combined with hardcoded exclusions.
3. 100ms debounce — rapid editor saves collapse into single events.
4. Relative path storage — `recordWatchEvent` stores paths relative to workspace root for consistent overlap matching.
5. PollWatcher retained as code but daemon now uses NativeWatcher.

New files: `internal/activity/native_watcher.go`, `internal/activity/ignore.go` with 11 unit tests.

Smoke tested end-to-end: file change detection <2s, .gitignore filtering (bin/ excluded), .paneignore filtering, relative path storage, real overlap detection, debounce.

Exit criteria met: file activity updates are near-instant and don't include noise from build artifacts or dependencies.

---

**V2 implementation is complete.** All six phases (V2.1–V2.6) are implemented and smoke tested.

### V2 dogfood findings — real panes/default daemon

Status: resolved; ready for final real-pane confirmation.

Original dogfood findings:

1. Native watcher worked: recent files appeared quickly on the board.
2. Attribution was not balanced for the overlap test: `tmp-pane-dogfood/overlap.txt` appeared under `test-agent`, but not under `auth-agent`.
3. Board/summary did not show the expected overlap indicator for `tmp-pane-dogfood/overlap.txt`.
4. `pane git status` produced normal git output with no warning, which is acceptable because status is non-risky.
5. `pane git rebase main` produced branch/rebase warnings, but did not include the expected overlap-specific file warning for `tmp-pane-dogfood/overlap.txt`.
6. `pane close` succeeded. A reported WebSocket error likely came from the surrounding agent/UI transport, not Pane itself; Pane uses Unix socket request/response, not WebSockets.

Root cause:

- watcher attribution selected a single owner when multiple active sessions shared the same best cwd prefix, so overlap could not be derived.

Fix:

- ambiguous same-cwd file events are now recorded for all equally likely sessions with low confidence; more specific cwd matches still win when one session is closer to the file.

Validation after fix:

- automated tests cover shared-cwd multi-session attribution and most-specific-cwd attribution
- temp-daemon smoke test showed `tmp-pane-dogfood/overlap.txt` under both sessions and board overlap output
- temp-daemon `pane git rebase main` showed overlap-specific preflight warning for `tmp-pane-dogfood/overlap.txt`

Recommended final check before V3:

- rerun the user's original two-pane overlap dogfood on the real default daemon/DB and confirm board/summary/git warnings match the temp-daemon smoke test

## Testing plan

### Automated baseline

Run before meaningful commits:

```bash
go test ./...
go vet ./...
make build
```

### Current smoke test

Use temp DB/socket paths so local state is not polluted:

```bash
make build

db="$(mktemp -t pane-smoke-db).sqlite"
sock="$(mktemp -u -t pane-smoke-sock).sock"

PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane daemon start &
pid=$!

for i in $(seq 1 30); do
  [ -S "$sock" ] && break
  sleep 0.1
done

PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane init
PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane intent "testing local board"
PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane status
PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane board
PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane summary
PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane daemon stop
wait "$pid"
```

### Messaging smoke test

Simulate two panes by overriding `ZELLIJ_PANE_ID`:

```bash
db="$(mktemp -t pane-msg-db).sqlite"
sock="$(mktemp -u -t pane-msg-sock).sock"

PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane daemon start &
pid=$!
for i in $(seq 1 30); do [ -S "$sock" ] && break; sleep 0.1; done

ZELLIJ_PANE_ID=1 PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane init
ZELLIJ_PANE_ID=2 PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane init

session_b=$(ZELLIJ_PANE_ID=2 PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane status | awk '/^session:/ {print $2}')

ZELLIJ_PANE_ID=1 PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane ask "$session_b" "Are you done?"
ZELLIJ_PANE_ID=2 PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane inbox

# copy a message id from inbox, then:
ZELLIJ_PANE_ID=2 PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane reply <message-id> "Yes"
ZELLIJ_PANE_ID=1 PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane inbox

PANE_DB_PATH="$db" PANE_SOCKET_PATH="$sock" ./bin/pane daemon stop
wait "$pid"
```

## Dogfooding plan

Start using Pane now with multiple panes.

The immediate dogfooding question:

> Does board + summary + visible message state reduce how much context the human has to hold?

Collect notes on:

- whether agents update intent reliably
- whether summaries are too thin or too noisy
- whether session IDs are too hard to target
- whether inbox delivery semantics feel right
- whether board output is useful at a glance
- what agents should know to run without being prompted

## Commit checklist

Before committing:

- `go test ./...`
- `go vet ./...`
- `make build`
- update `PROGRESS.md` if phase status changed
- update `ARCHITECTURE.md` if flow or ownership changed
- keep the README aligned with the product framing
