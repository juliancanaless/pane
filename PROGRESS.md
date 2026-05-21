# Pane Progress

This file is intentionally committed so every machine and every agent can see the current plan, what exists, and what should happen next.

## Current status

Pane has an early but working daemon-backed core.

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
- `pane daemon health`
- `pane daemon stop`
- Unix socket request/response protocol
- SQLite opened once by the daemon

### Sessions

- `pane init`
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

## What is not real yet

These are important but not implemented yet:

- file working-set overlap detection
- daemon auto-start outside shell hook
- PID/lock/log lifecycle
- richer session lineage beyond first parent links
- richer `pane history` filters and summaries
- richer generic `pane state` workflows beyond first-pass key/value JSON

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

Status: first pass implemented.

Goal:

Make shell/agent participation less noisy and less dependent on re-running full session init.

Completed:

1. Added daemon-backed `pane heartbeat`.
2. Heartbeat refreshes cwd, branch, tty, last seen, and active status while preserving current intent.
3. Heartbeat creates/resumes a session if none exists for the pane/workspace, preserving shell-hook convenience.
4. Updated `pane shell-init` to use `pane heartbeat` instead of quietly re-running `pane init` on every prompt.

Still needed:

- daemon PID/lock/log lifecycle hardening
- auto-start behavior outside shell hook
- decide whether long-running agent commands need activity signals beyond prompt heartbeat

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
