# Pane Architecture

Pane is the local persistence and awareness layer for agent work.

The short version:

```text
agent / human runs pane command
        ↓
short-lived pane CLI
        ↓ length-prefixed JSON over Unix socket
long-running local Pane daemon
        ↓
SQLite-backed shared memory
```

The daemon is the source of truth. The CLI should stay thin: detect local context, send a request, print the daemon's response.

## Product model

Pane is not primarily a git tool, a dashboard, or an orchestrator. The concrete workflows Pane is meant to solve are documented in `USE_CASES.md`.

Pane is an environment layer for agents:

- **Sequential continuity**: a new session can inherit what previous sessions did.
- **Concurrent awareness**: active sessions can see what other sessions are doing.
- **Persistent agent memory**: future agent-specific state can live in one local store instead of bespoke per-agent caches.

V1 focuses on concurrent awareness because it is the right foundation: sessions, intents, board, summaries, messages, file activity, and git guardrails.

## Core components

### `cmd/pane`

Single binary entry point.

It exposes commands such as:

- `pane daemon start|status|health|stop`
- `pane init`
- `pane heartbeat`
- `pane status`
- `pane intent <text>`
- `pane board`
- `pane summary`
- `pane continue <session-id>`
- `pane history [--since <duration>]`
- `pane ask <session-id> <message>`
- `pane inbox`
- `pane reply <message-id> <message>`
- `pane state set|get|list|delete ...`
- `pane git <args...>`

### CLI layer: `internal/cli`

The CLI is short-lived and stateless.

Responsibilities:

1. parse command arguments
2. detect environment when needed:
   - pane id from Zellij/tmux/TTY
   - workspace root
   - cwd
   - branch
3. send a protocol request to the daemon
4. render the daemon response

The CLI should not own coordination state. Session, board, summary, and messaging commands go through the daemon.

### Protocol layer: `internal/protocol`

The CLI and daemon communicate over a Unix socket using:

```text
4-byte big-endian message length + JSON payload
```

Current request families:

- daemon health/status/stop
- session init/heartbeat/status/intent/continue/history
- board/summary
- message send/list/reply
- generic state set/get/list/delete
- git preflight/record

### Daemon layer: `internal/daemon`

The daemon is the local long-running coordinator.

Responsibilities:

- open SQLite once
- publish first-pass lifecycle metadata through health/status: pid, socket, DB, PID file, and log path
- own session manager and message store
- handle CLI requests
- generate board and summary views
- run file watchers
- evaluate git preflight risk
- maintain activity/history/state APIs

The daemon is what makes Pane shared memory rather than a collection of disconnected commands.

### Store layer: `internal/store`

SQLite persistence.

Current tables:

- `sessions`
- `messages`
- `file_activity`
- `git_events`
- `agent_state`

Current stores:

- session store, including parent-session lineage and history queries
- message store
- file activity store
- git event store
- generic namespaced state store

Future stores:
- richer lineage/history stores

### Session layer: `internal/session`

A Pane session is tied to the terminal pane/workspace, not to an agent process.

Identity priority:

1. `$ZELLIJ_PANE_ID`
2. `$TMUX_PANE`
3. TTY fallback

This lets a new agent in the same pane resume the previous pane context.

### Board layer: `internal/board`

The board is the workspace-wide shared awareness view.

It answers:

- which sessions are active or idle?
- what is each session trying to do?
- where is each session working?
- what should another agent know before acting?

Current board data:

- session id and short id
- status
- branch
- cwd
- intent
- last seen
- unread message count
- awaiting reply count
- recent files

Future board data:

- hot directories
- overlaps
- recent guardrail events

### Summary layer: `internal/summary`

`pane summary` is the current-session startup/resume view.

It is narrower than `pane board` and oriented around:

- who am I?
- what was this pane doing?
- who else is nearby?
- what context should I load before acting?

Current summaries include current session, peer sessions, unread messages, awaiting reply counts, recent files for the current session, and first-pass continuity context from parent/recent sessions.

Future summaries should include unresolved decisions, hot directories, overlap, lineage chains, and richer activity history.

### Messages layer: `internal/messages`

Explicit async coordination between sessions.

Current flow:

```bash
pane ask <session-id> "question"
pane inbox
pane reply <message-id> "answer"
```

Messages remove the human as the context router.

### Git guard layer: `internal/gitguard`

V1 includes git as an early guardrail, not as the product center.

`pane git` currently:

1. parses watched git commands
2. asks the daemon for preflight risk
3. prints concise first-pass warnings
4. executes real git preserving stdout/stderr/exit behavior
5. records the git event

Future preflight should include file activity overlap and more nuanced branch/remote checks.

## Current command flow

Example: `pane intent "working on auth"`

```text
CLI detects pane/workspace
  → sends SessionIntent request
    → daemon resolves current session
      → daemon updates SQLite
        → daemon returns updated intent
          → CLI prints confirmation
```

Example: `pane board`

```text
CLI detects workspace
  → sends GetBoard request
    → daemon lists active/idle sessions
      → daemon renders board
        → CLI prints board
```

Example: `pane ask session-b "Are you done?"`

```text
CLI detects current session
  → sends MessageSend request
    → daemon stores queued message
      → target session sees it via pane inbox
```

## V1 build order

1. Session identity and lifecycle — done
2. Daemon-backed board — done
3. Daemon-backed summary — done
4. Daemon-backed messaging — done
5. Surface message state in board/summary — done
6. File activity and working sets — first pass done
7. Git passthrough and preflight — first pass done
8. Shell/agent integration — first pass done
9. Sequential continuity — first pass done
10. Generic agent state — first pass done
11. Heartbeat hardening — first pass done
12. Daemon lifecycle hardening — first pass done
13. Targeting ergonomics — first pass done

## V2/V3 direction from the reframe

The `REFRAMING.md` direction widens the lens without changing the V1 foundation.

V2 starts sequential continuity:

- session lineage via parent session links
- `pane continue`
- richer `pane summary` from history
- `pane history` for work tracking

V3 starts generic agent memory:

- `pane state set/get/list/delete`
- workspace-scoped namespaced JSON state
- future integrations for specialized agents such as Neon/APM

## Shell and agent integration

`pane shell-init` prints shell code for bash/zsh that:

- starts the daemon if needed
- initializes/resumes the pane session
- prints a startup summary
- runs `pane heartbeat` on each prompt to refresh cwd, branch, and last-seen state without re-running full init output

`pane shims install` creates a git shim at `~/.pane/shims/git` so ordinary `git` commands can route through `pane git` when that directory is prepended to PATH.

`AGENTS.md` gives coding agents the operating contract for this repo.

## Design rules for future agents

When modifying Pane, preserve these rules:

1. The daemon owns shared state.
2. The CLI is thin.
3. Agents update their own intent/status.
4. Humans can inspect, but should not maintain the board manually.
5. Board/summary should reduce context burden, not add noise.
6. Git is a guardrail surface, not the product center.
7. Prefer accumulating structured context as work happens over reconstructing handoffs after the fact.
