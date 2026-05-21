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
- daemon-backed

### Summary

- `pane summary`
- current-session startup/resume view
- shows current session and peer sessions
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

- message state surfaced inside `pane board`
- message state surfaced inside `pane summary`
- file activity watcher
- file working-set / overlap detection
- `pane git` real passthrough
- git preflight warnings
- git event storage
- shell integration / `pane shell-init`
- daemon auto-start
- PID/lock/log lifecycle
- session lineage / `pane continue`
- `pane history`
- generic `pane state` API

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

Status: next.

Goal:

Agents should not need to remember to run `pane inbox` to know coordination state exists. Board and summary should surface message state.

Tasks:

1. Add message store queries for:
   - unread count by session
   - queued messages for current session
   - open outbound threads from current session
2. Add a coordination section to `pane summary`:
   - unread messages
   - sent questions waiting for reply
3. Add compact message indicators to `pane board`:
   - unread count per session
   - maybe open outbound count per session
4. Keep output concise; do not dump full message bodies in board.

Exit criteria:

- after `pane ask`, target session sees a visible unread indicator in summary/board
- `pane inbox` still shows full message bodies
- board remains compact

### Phase 5 — file activity and working sets

Status: not started.

Goal:

The board reflects observed work, not just declared intent.

Tasks:

1. Implement file activity store.
2. Choose watcher dependency and platform path.
3. Filter `.git/`, ignored files, build artifacts.
4. Attribute activity heuristically.
5. Derive recent files, hot directories, and overlap.
6. Show recent activity and overlap in board/summary.

### Phase 6 — git guardrail

Status: parser only.

Goal:

`pane git ...` behaves like real git while adding shared-state preflight warnings for risky commands.

Tasks:

1. Real git passthrough preserving stdout/stderr/exit code.
2. Daemon `GitPreflight` handler.
3. Daemon `GitRecord` handler.
4. Narrow command-specific warnings.
5. Daemon-down behavior should warn and continue for git.

### Phase 7 — shell/agent integration

Status: not started.

Goal:

Agents naturally participate in Pane without humans manually driving every update.

Tasks:

1. `pane shell-init`
2. heartbeat hooks
3. git shim generation
4. agent instruction snippet / AGENTS.md guidance
5. dogfood with real agents across multiple panes

### Phase 8 — sequential continuity

Status: future.

Goal:

Replace manual handoffs with accumulated session lineage.

Tasks:

1. session lineage model
2. `pane continue <session-id>`
3. summary from previous sessions
4. decisions/open threads/history in summary
5. `pane history --since ...`

### Phase 9 — generic agent state

Status: future.

Goal:

One local persistence API for specialized agents.

Potential commands:

```bash
pane state set <namespace.key> <json>
pane state get <namespace.key>
pane state list <namespace-prefix>
pane state delete <namespace.key>
```

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

> Does board + summary + messaging reduce how much context the human has to hold?

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
