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
        ↓
Rust/tree-sitter analyzer subprocess for semantic analysis
```

The daemon is the source of truth. The CLI should stay thin: detect local context, send a request, print the daemon's response.

## Product model

Pane is not primarily a git tool, a dashboard, or an orchestrator. The concrete workflows Pane is meant to solve are documented in `USE_CASES.md`.

Pane is an environment layer for agents:

- **Sequential continuity**: a new session can inherit what previous sessions did.
- **Concurrent awareness**: active sessions can see what other sessions are doing.
- **Worktree-aware codebase awareness**: sessions in separate Git worktrees can remain isolated while still coordinating as one repository.
- **Persistent agent memory**: future agent-specific state can live in one local store instead of bespoke per-agent caches.

### Workspace root vs repository identity

Pane currently treats `workspace_root` as the primary scope. That is correct for file watching, cwd rendering, and local indexing because each Git worktree has its own filesystem root. It is incomplete for Done-quality multi-agent work because sibling worktrees can represent the same repository.

The model is two-layered:

- `workspace_root`: concrete checkout/worktree path; owns watcher events, cwd, local file activity, and local analyzer indexing.
- `repo_id`: stable Git common-dir based repository identity shared by related worktrees; owns cross-worktree board/history aggregation and branch-risk checks.

Current first pass stores `repo_id` and `git_common_dir` on sessions, exposes `pane board --repo` and `pane history --repo`, and lets git preflight consider active sessions in sibling worktrees. Semantic graph normalization across worktrees is still future work.

Git is optional. Outside a repository, `workspace_root` falls back to `PANE_WORKSPACE_ROOT` or the working directory and `repo_id` stays empty, which is the single flag the rest of the system keys on: git guardrails and `--repo` scope are unavailable, and shell-hook heartbeats refresh existing sessions but never create one (only an explicit `pane init` does), so passing through arbitrary directories leaves no sessions behind.

### Daemon version handshake

The daemon stamps its release version on every response. A CLI that receives a response from an older daemon (or a pre-versioning one that stamps nothing) restarts it in place with the current binary after serving the response — this is how `brew upgrade` reaches the long-running daemon without manual intervention. An older CLI never downgrades a newer daemon.

## Core components

### Analysis layer: `analysis/` and `internal/analysis`

V3 introduces a Rust analysis engine.

Current first pass:

- `analysis/` contains the Rust `pane-analyze` binary.
- `pane-analyze symbols <file>` parses Go, Python, Rust, TypeScript, and TSX with tree-sitter.
- `pane-analyze deps <file>` extracts first-pass imports/use/require dependency edges with confidence scores.
- Outputs are JSON symbol tables and dependency graphs.
- `internal/analysis.Client` calls the analyzer as a subprocess.
- `pane analyze symbols|deps <file>` exposes this through the Go CLI.
- `pane analyze index <path...>` persists symbols and dependency edges in SQLite.
- `pane analyze dependents <target>` queries persisted dependency edges for downstream files.
- The daemon also incrementally re-indexes supported source files from file watcher events.
- Board, summary, and git preflight use recent file activity plus persisted semantic data to surface first-pass semantic overlaps.

The subprocess boundary is intentional for the scaffold: it avoids CGo/FFI complexity and keeps Rust analysis independently testable. A later V3 phase can revisit FFI if latency requires it.

### `cmd/pane`

Single binary entry point.

It exposes commands such as:

- `pane daemon start|status|health|stop`
- `pane setup`
- `pane doctor`
- `pane init`
- `pane heartbeat`
- `pane status`
- `pane intent <text>`
- `pane board`
- `pane summary`
- `pane continue <session-id>`
- `pane spawn <command> [args...]`
- `pane history [--since <duration>] [--lineage] [--format work-log]`
- `pane board [--repo|--global]`
- `pane ask [--global] <session-id> <message>`
- `pane inbox`
- `pane reply <message-id> <message>`
- `pane state set|get|list|namespaces|delete ...`
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

The CLI should not own coordination state. Session, board, summary, and messaging commands go through the daemon. Setup/doctor are local installation helpers: `pane setup` copies the current binary and `pane-analyze` helper to `~/.pane/bin`, installs shell/git integration, and starts the daemon; setup flags such as `--no-shell`, `--no-shim`, `--no-daemon`, and `--print-shell` support safer package-manager or dotfile-managed installs. `pane doctor` checks expected paths, analyzer availability, daemon health, and platform identity. CI runs macOS/Linux tests, builds, and daemon smoke coverage.

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
- generic state set/get/list/namespaces/delete
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
- incrementally index changed source files for semantic analysis
- evaluate git preflight risk, including first-pass semantic overlap warnings
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
- `analysis_symbols`
- `dependency_edges`

Current stores:

- session store, including parent-session lineage and history queries; `pane history --lineage` renders known recent parent/child chains from this data, and `pane history --format work-log` combines sessions with file/git activity for first-pass work reports
- message store
- file activity store
- git event store
- generic namespaced state store; first-pass global state uses reserved workspace root `__global__`, and workspace `summary.*` keys are surfaced in session summaries
- analysis store for persisted symbol tables and dependency edges

Future stores:
- richer lineage/history stores

### Session layer: `internal/session`

A Pane session is tied to the terminal pane/workspace, not to an agent process. Spawned child commands can override that identity with `PANE_PANE_ID` and link to a parent with `PANE_PARENT_SESSION_ID`, which is how first-pass worker/child session hierarchies are represented without a separate hierarchy table.


Identity priority:

1. `$ZELLIJ_PANE_ID`
2. `$TMUX_PANE`
3. TTY fallback

This lets a new agent in the same pane resume the previous pane context. Sessions are durable SQLite records and survive daemon restarts. The default active board hides closed sessions and first-pass stale sessions; `pane close` explicitly closes the current session and `pane sessions prune` closes stale active/idle sessions in the workspace.

### Board layer: `internal/board`

The board is the workspace-wide shared awareness view.

It answers:

- which sessions are active or idle?
- what is each session trying to do?
- where is each session working?
- what should another agent know before acting?

Board scope is selectable: default workspace, `--repo` for sibling worktrees of
the same repository, and `--global` for every session on the machine across all
workspaces. The global board is a roster only — overlap, semantic, and git
signals are keyed to a single `workspace_root`, so they are omitted at machine
scope where they would be meaningless. Its purpose is discovery: find the full
session id of a peer in another workspace to `pane ask --global`.

Current board data:

- session id and short id
- status
- branch
- cwd
- intent
- last seen
- unread message count
- awaiting reply count
- recent files in the full-detail decay tier (<5m)
- decayed activity summaries for older file activity (5m–72h)
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

Current summaries include current session, peer sessions, unread messages, awaiting reply counts, full-detail recent files for the current session, decayed activity summaries, and first-pass continuity context from parent/recent sessions.

Future summaries should include unresolved decisions, richer lineage chains, and persisted natural-language activity summaries.

### Messages layer: `internal/messages`

Explicit async coordination between sessions.

Current flow:

```bash
pane ask <session-id> "question"
pane inbox
pane reply <message-id> "answer"
```

Messages remove the human as the context router.

Messages are stored and delivered by global session id, not by workspace — the
machine runs a single daemon and database, so the message plane is already
machine-wide. By default `pane ask` only *resolves* targets within the caller's
workspace. `pane ask --global <full-session-id>` lifts that restriction so a
session can message any other session on the machine; an exact full session id
always resolves, while names and short ids stay workspace-scoped to avoid
cross-machine collisions. Discover foreign sessions with `pane board --global`.

### Git guard layer: `internal/gitguard`

V1 includes git as an early guardrail, not as the product center.

Worktree note: git guardrails reason across sibling worktrees of the same repository for first-pass branch risk. Worktrees are a safety mechanism, not a separate product scope; Pane should warn when branch or semantic risk crosses worktree boundaries while preserving each worktree's separate working directory. Semantic cross-worktree risk remains a later refinement.

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
14. Session lifecycle cleanup / board freshness — first pass done

## V2/V3/Done direction

The full guided plan lives in `ROADMAP.md`. Summary:

| Version | Theme | Key capability |
|---------|-------|---------------|
| **V1** ✅ | Reliable awareness | Sessions, board, messaging, git guardrails, file activity, agent state |
| **V2** | Deep coordination | Overlap detection, richer preflight, daemon hardening, board/summary signal quality |
| **V3** | Semantic intelligence | Rust analysis engine, tree-sitter parsing, dependency graphs, symbol-level overlap |
| **Done** | Production infrastructure | Worker hierarchies, installer, work tracking, platform hardening |

V2 deepens the V1 surface with file-level overlap, richer git warnings, and daemon reliability.

V3 introduces a Rust analysis engine for symbol-level dependency graphs and semantic diffing.

Done means Pane is infrastructure you forget about.

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
