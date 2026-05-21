# Pane — Technical Architecture (V1)

## 1. Overview

Pane is a local coordination layer for multi-agent development workflows. It runs entirely on the developer's machine, requires no network access, and works with any AI coding agent that executes shell commands.

V1 scope is deliberately narrow. Its technical features exist to support the product use cases documented in [`../USE_CASES.md`](../USE_CASES.md): agent restart continuity, cross-pane handoff, concurrent agent awareness, human handoff relief, workspace memory, safer high-risk operations, specialized agent memory, and provider-agnostic collaboration.

Pane does six things:

1. Track sessions by terminal pane, not by agent process
2. Maintain a shared awareness board for active sessions in a workspace
3. Track file-level working sets and detect overlap between sessions
4. Inject context at high-leverage moments (session start, git preflight)
5. Route explicit async messages between sessions
6. Intercept git commands and warn before risky shared-state operations

This document specifies the architecture for that scope.

---

## 2. System components

Pane V1 consists of two components:

### 2.1 `pane` — the single binary

A single Go binary that serves as both the CLI and the daemon.

As a **daemon** (`pane daemon start`):

- listen on a Unix domain socket for requests from CLI subcommands
- manage session lifecycle (create, resume, heartbeat, close)
- ingest and store file activity events from the file watcher
- evaluate git preflight checks against current session state
- store and route inter-session messages
- generate session-start summaries and preflight warnings
- persist all state to a local SQLite database

The daemon is the single source of truth. All coordination flows through it.

The daemon also owns the shared awareness board: the queryable view of active sessions, their current intent, recent activity, overlap, messages, and recent guardrail events.

As a **CLI** (`pane git`, `pane ask`, `pane init`, etc.):

- wrap `git` subcommands: send command intent to the daemon, print any warnings, then execute the real git binary
- expose messaging subcommands: `ask`, `inbox`, `reply`
- expose session subcommands: `status`, `intent`, `init`
- fetch and print session-start summaries on demand

CLI invocations are stateless. Every invocation makes a request to the daemon over the Unix socket, acts on the response, and exits. The round-trip target is under 5ms for non-blocking operations.

### 2.2 File watcher

A goroutine within the daemon (not a separate process) that watches the workspace filesystem.

Responsibilities:

- watch for file create/write/delete events using platform-native APIs (FSEvents on macOS, inotify on Linux)
- attribute each event to a session using heuristic correlation (see section 6.2)
- record attributed events in the `file_activity` table
- respect `.gitignore` and a `.paneignore` file for filtering noise

The watcher does not need perfect attribution in V1. Neither FSEvents nor inotify provide the PID of the writing process, so attribution relies on heuristics: which session's `cwd` is closest to the changed file, which session most recently ran a command, and recency tiebreaking. If attribution confidence is low, the event is still recorded but flagged.

---

## 3. Communication

### 3.1 CLI ↔ daemon

All communication uses a Unix domain socket at a well-known path:

```
~/.pane/pane.sock
```

The protocol is request/response over the socket using length-prefixed JSON messages. No HTTP overhead, no gRPC dependency. Just:

```
[4-byte big-endian length][JSON payload]
```

This keeps the implementation simple and the latency minimal.

### 3.2 Request types

Every request from a CLI subcommand to the daemon includes a `session_id` (or enough info to resolve one: pane ID, TTY, workspace root).

| Request | Purpose |
|---|---|
| `SessionInit` | Register or resume a session for this pane/TTY/workspace |
| `SessionHeartbeat` | Update last_seen, cwd, branch |
| `SessionClose` | Mark session inactive |
| `SessionStatus` | Get current session info |
| `SessionIntent` | Update the session's stated intent |
| `GitPreflight` | Check safety before a git command |
| `GitRecord` | Record that a git command completed |
| `GetBoard` | Fetch workspace-wide shared awareness board |
| `GetSummary` | Fetch session-start summary for this session |
| `MessageSend` | Send a message to another session |
| `MessageList` | Get inbox for this session |
| `MessageReply` | Reply to a specific message |

### 3.3 Response shape

Every response includes:

```json
{
  "ok": true,
  "warnings": [],
  "block": false,
  "payload": {}
}
```

- `ok`: whether the request succeeded
- `warnings`: array of human-readable warning strings to print to stderr
- `block`: whether the operation should be blocked (only for dangerous git ops)
- `payload`: request-specific data

---

## 4. Session identity

### 4.1 Identity model

A session is identified by the tuple:

```
(pane_id or tty, workspace_root)
```

When `pane init` runs, it sends the current pane ID (from `$ZELLIJ_PANE_ID`, `$TMUX_PANE`, or the TTY path from `tty`) and the workspace root (from `git rev-parse --show-toplevel`).

Pane requires a git repository. If `git rev-parse --show-toplevel` fails, `pane init` prints an error and exits. Pane's entire value proposition is git coordination — there is nothing useful to do outside a repo.

The daemon checks for an existing session matching that tuple with `status = active` or `status = idle` and applies the following resume logic:

- **Resume if**: same pane/TTY, same workspace, `last_seen_at` within 4 hours, and current branch matches the session's recorded branch
- **New session if**: the resume window has expired, or the branch has changed since the session went idle (indicating the work context shifted)

This handles common gaps like lunch breaks and meetings without resuming stale sessions from the previous day or a different task.

On resume, the daemon returns accumulated context (summary, unread messages, coordination state). On new session, it still returns awareness of other active sessions in the same workspace.

### 4.2 Session lifecycle

```
    ┌──────────┐
    │  created  │  pane init
    └────┬─────┘
         │
    ┌────▼─────┐
    │  active   │◄──── pane heartbeat / any pane command
    └────┬─────┘
         │ no activity for idle_timeout (default 5 min)
    ┌────▼─────┐
    │   idle    │  still resumable (up to 4 hours, if branch matches)
    └────┬─────┘
         │ no activity for close_timeout (default 4 hours) or branch changed
    ┌────▼─────┐
    │  closed   │  archived, not resumable
    └──────────┘
```

Every `pane` CLI invocation implicitly heartbeats the session, updating `last_seen_at`, `cwd`, and `branch`.

### 4.3 Pane detection

Priority order for identifying the pane:

1. `$ZELLIJ_PANE_ID` — if running inside Zellij
2. `$TMUX_PANE` — if running inside tmux
3. output of `tty` command — fallback for raw terminals

The pane identifier is stable across agent restarts within the same terminal pane, which is the key property that enables session continuity.

### 4.4 Agent operating contract

Pane is designed for agents to keep the shared board current themselves. Humans can inspect or override state, but they should not be the coordination bus.

A Pane-aware agent should:

1. Run `pane init` when starting or resuming in a terminal pane.
2. Read `pane summary` before beginning work.
3. Set intent with `pane intent "<current task>"` before starting meaningful work.
4. Update intent whenever it switches tasks or changes scope.
5. Check `pane inbox` when beginning work and after receiving summary context.
6. Use `pane ask <session-id> "..."` when it needs context from another session.
7. Use `pane reply <message-id> "..."` to answer coordination questions.
8. Run git through `pane git` or a PATH shim so risky operations receive preflight checks.
9. Treat Pane warnings as shared-state context to reason about, not as ordinary terminal noise.

This contract is critical. If agents do not update intent/status and read the board themselves, the human is forced back into manually tracking and relaying context, which is exactly what Pane is meant to remove.

---

## 5. Shared awareness board

The shared awareness board is the daemon-maintained view of what active sessions are doing in a workspace.

It is not a dashboard, task planner, lock registry, or territory map. It is shared local memory for concurrent agents.

The board answers:

- which sessions are active in this workspace?
- what is each session currently trying to do?
- what files or directories has each session touched recently?
- where do working sets overlap?
- what coordination messages are unread or unresolved?
- what recent git/guardrail events may affect this session?

### 5.1 Board inputs

The board is assembled from:

- `sessions`: pane identity, workspace, cwd, branch, status, last intent, heartbeat timestamps
- `file_activity`: recent file events attributed to sessions
- `messages`: unread messages, open threads, recently resolved threads
- `git_events`: recent guarded git operations and outcomes

### 5.2 Board outputs

The board is surfaced through:

- `pane summary`: broad startup/resume orientation
- `pane status`: current session state
- `pane board`: workspace-wide view of all active sessions
- `pane git`: narrow command-specific preflight warnings
- `pane inbox`: direct coordination messages

### 5.3 `pane board`

V1 should expose an explicit board command in addition to startup summaries:

```bash
pane board
```

This command should show a compact workspace-wide view:

```text
[Pane] Workspace board
Active sessions: 3

Session session-a12 — active — feature/auth
  Intent: refactoring auth middleware
  CWD: src/auth
  Recent: src/auth/token.ts, src/auth/session.ts

Session session-b44 — active — feature/auth
  Intent: writing auth tests
  CWD: tests/auth
  Recent: tests/auth/token.test.ts
  Overlap: shares auth hot directory with session-a12

Session session-c91 — idle — feature/payments
  Intent: investigating Stripe webhook retry behavior
  Recent: src/payments/webhooks.ts

Coordination:
  Unread: session-b44 asked session-a12 whether auth/session.ts is safe to modify
  Open: session-a12 waiting on session-c91 about payment fixture ownership
```

The board should be concise. It should summarize current coordination state rather than replay raw logs.

---

## 6. Git interception

### 6.1 How `pane git` works

When the user or agent runs `pane git <subcommand> [args...]`:

1. `pane` parses the git subcommand
2. if the subcommand is in the watched set, `pane` sends a `GitPreflight` request to the daemon
3. the daemon evaluates the request against current session state
4. the daemon returns warnings and/or a block directive
5. `pane` prints warnings to stderr
6. if blocked, `pane` prompts for confirmation
7. if proceeding, `pane` executes the real `git` binary with the original arguments
8. `pane` sends a `GitRecord` request with the outcome

### 6.2 Watched git subcommands

| Subcommand | Risk | What to check |
|---|---|---|
| `checkout` / `switch` | moving onto a branch with active sessions | other sessions on target branch, uncommitted overlap |
| `commit` | publishing local state | overlapping file activity from other sessions |
| `pull` | integrating remote changes during active local work | other sessions on same branch with uncommitted work |
| `push` | publishing to remote | other sessions with unpushed commits on same branch |
| `merge` | combining branches | active sessions on source or target branch |
| `rebase` | rewriting history | other sessions on same branch, uncommitted overlap |
| `reset --hard` | destructive local state change | recent local file activity, overlap with other sessions |

### 6.3 Preflight evaluation

For each watched command, the daemon runs a checklist:

1. **branch overlap**: are other active sessions on the same branch (source or target)?
2. **file overlap**: do other sessions have recent file activity in paths that this session also touched?
3. **uncommitted risk**: does this command risk discarding or conflicting with uncommitted work in other sessions?
4. **recency**: how recent is the overlapping activity? (more recent = more urgent warning)

The evaluation returns one of:

- **pass**: no relevant overlap, proceed silently
- **warn**: overlap detected, print warning, proceed unless user cancels
- **block**: high-risk operation (e.g., `push --force` with active sessions on same branch), require explicit `y` confirmation

### 6.4 PATH-based interception (optional)

For environments where aliasing `git` to `pane git` is inconvenient, Pane can create a shim directory:

```
~/.pane/shims/git  →  pane git "$@"
```

Prepending `~/.pane/shims` to `$PATH` makes interception transparent to agents without modifying their configuration.

---

## 7. File activity tracking

### 7.1 Filesystem watching

The daemon uses a platform-native filesystem watcher to observe the workspace root recursively.

On macOS, FSEvents provides inherently recursive directory watching. On Linux, inotify requires manually adding watches for each subdirectory and handling new directory creation. The Go library `rjeczalik/notify` abstracts this difference and handles recursive watching correctly on both platforms. This is preferable to `fsnotify`, which does not support recursive watching natively and requires manual directory tree walking.

Events captured:

- file created
- file modified
- file deleted
- file renamed

Filtered out:

- `.git/` directory changes (git's own internal operations)
- paths matching `.gitignore` patterns
- paths matching `.paneignore` patterns
- binary files and build artifacts

### 7.2 Session attribution

Neither FSEvents (macOS) nor inotify (Linux) provide the PID of the process that wrote a file. This means process-tree-based attribution — walking from the writing PID up to a known shell PID — is not available through standard filesystem watching APIs.

In V1, attribution uses heuristics:

1. **cwd proximity**: which session's current working directory is closest to the changed file's path?
2. **recency**: which session most recently ran a command (via `pane git` or heartbeat)?
3. **branch affinity**: if the file is in a path associated with a specific branch's recent changes, prefer the session on that branch
4. **explicit claim**: if a session recently ran `pane git` or `pane intent` referencing files in this area, prefer that session

Attribution includes a confidence field: `high`, `medium`, `low`.

- **high**: single active session in the workspace, or file is within a session's explicitly claimed working set
- **medium**: multiple sessions active but cwd proximity and recency agree
- **low**: multiple sessions active with ambiguous signals

Low-confidence attributions are stored but do not trigger warnings to other sessions. This avoids false alerts while still recording activity for debugging.

Future versions may use platform-specific APIs for PID-based attribution (Endpoint Security framework on macOS, `fanotify` with `FAN_REPORT_FID` on Linux), but these require elevated permissions and are out of scope for V1.

### 7.3 Working set derivation

Each session's working set is derived from recent `file_activity` rows:

- **recent files**: files with activity in the last N minutes (default 15)
- **hot directories**: directories containing 2+ recently active files
- **overlap score**: for any pair of sessions within the same workspace, the count of shared recent files or shared hot directories

The overlap score powers both startup summaries and preflight warnings.

Overlap is always scoped to a workspace. Sessions in different workspaces never overlap, even if they are in adjacent panes. This prevents noise from unrelated projects.

---

## 8. Context injection

### 8.1 Session-start summary

Generated by the daemon when `pane init` or `pane summary` is called.

Contents:

| Section | Source |
|---|---|
| Current branch | session record |
| Recent files this session touched | file_activity for this session |
| Other active sessions | sessions table |
| Each session's branch and recent files | sessions + file_activity |
| Overlap warnings | working set overlap computation |
| Coordination state | messages table, filtered and summarized |

The coordination state section includes:

- **Unread messages**: direct inbound messages not yet delivered
- **Open threads**: questions sent by this session that have no reply yet
- **Recently resolved threads**: replies received since last session start that may affect current assumptions

This gives a new agent full working memory for the pane without replaying raw message history.

#### Output format

```
[Pane] Session resumed — branch: feature/auth

Active sessions:
  Session 1: feature/auth — src/auth/token.ts, src/auth/session.ts
  Session 3: feature/payments — src/payments/stripe.ts

Overlap:
  Session 1 edited tests/auth/token.test.ts 3 min ago

Coordination:
  Open: Waiting on Session 1 to confirm token refresh behavior
  Resolved: Session 1 confirmed validateToken() returns Result<Token, AuthError>
  Unread: Session 3 asked whether auth/session.ts is safe to modify
```

The summary is printed to stderr so it does not interfere with agent tool output parsing.

### 8.2 Git preflight warnings

Generated by the daemon as part of the `GitPreflight` response.

Only included when there is meaningful shared-state risk. If there is no overlap, the response contains no warnings and `pane git` proceeds silently.

Contents are command-specific and narrow:

- which session is relevant
- which branch or files are implicated
- how recent the overlap is
- why this specific command is risky

#### Output format

```
[Pane] Heads up:
  Session 2 is active on feature/auth
  Touched src/auth/token.ts 3 min ago
  Has uncommitted activity in tests/auth/token.test.ts

  You're about to run: git rebase main
  This may collide with overlapping in-progress work.
  Proceed? [y/N]
```

---

## 9. Inter-session messaging

### 9.1 Message model

Messages are simple text payloads routed between sessions.

A message has:

- `message_id`: unique identifier (ULID for time-sortable IDs)
- `thread_id`: groups conversation exchanges (the first message's `message_id` becomes the `thread_id` for the entire thread)
- `from_session_id`: sender
- `to_session_id`: recipient (or `*` for broadcast)
- `body`: text content
- `state`: `queued` → `delivered` | `expired`
- `created_at`: timestamp
- `delivered_at`: when it was included in a summary or inbox response

Replies are stored as separate message rows with the same `thread_id`. This supports multi-turn exchanges:

```
msg-001: Session A → Session B  "Is auth/session.ts safe to modify?"  (thread_id: msg-001)
msg-002: Session B → Session A  "Not yet, give me 5 minutes"          (thread_id: msg-001)
msg-003: Session B → Session A  "Ok, done now"                        (thread_id: msg-001)
```

Thread state is derived, not stored:

- **open**: the most recent message in the thread is from the original sender (still waiting for a response)
- **active**: back-and-forth in progress
- **resolved**: the most recent message is from the recipient, and no further messages for a configurable window (default: 30 minutes)
- **expired**: no activity for 2 hours

### 9.2 CLI commands

```bash
# send a question to another session
pane ask <session-id> "Is auth/session.ts safe to modify?"

# view inbox
pane inbox

# reply to a message
pane reply <message-id> "Yes, I'm done with that file."
```

### 9.3 Delivery

Messages are delivered at two points:

1. **Session-start summary**: unread messages and open threads are included in the coordination section
2. **Inbox command**: `pane inbox` returns all unread messages for the current session

Messages are marked `delivered` when they are included in either a summary or inbox response. This prevents duplicate delivery.

### 9.4 Thread tracking

When session A sends a message to session B, a thread is created. The thread appears as "open" in session A's coordination summary until session B replies.

When session B replies via `pane reply`, a new message row is created in the same thread. The reply appears in session A's next summary as a "recently resolved" item.

Threads expire after a configurable window (default: 2 hours) if no activity occurs.

---

## 10. Data model

All state lives in a SQLite database at:

```
~/.pane/pane.db
```

SQLite is configured with WAL mode for concurrent read/write performance.

### 10.1 Schema

```sql
CREATE TABLE sessions (
    session_id     TEXT PRIMARY KEY,
    pane_id        TEXT NOT NULL,
    tty            TEXT,
    workspace_root TEXT NOT NULL,
    cwd            TEXT,
    branch         TEXT,
    last_intent    TEXT,
    started_at     INTEGER NOT NULL,
    last_seen_at   INTEGER NOT NULL,
    status         TEXT NOT NULL DEFAULT 'active'
);

-- No UNIQUE constraint on (pane_id, workspace_root).
-- Multiple closed sessions for the same pane/workspace coexist.
-- Session resume uses a filtered query:
--   WHERE pane_id = ? AND workspace_root = ? AND status IN ('active', 'idle')
--   AND last_seen_at > unixepoch() - 14400   -- 4 hour window
CREATE INDEX idx_sessions_lookup ON sessions(pane_id, workspace_root, status);

CREATE TABLE file_activity (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id    TEXT NOT NULL REFERENCES sessions(session_id),
    path          TEXT NOT NULL,
    event_type    TEXT NOT NULL,
    attribution   TEXT NOT NULL DEFAULT 'high',
    timestamp     INTEGER NOT NULL
);

CREATE INDEX idx_file_activity_session ON file_activity(session_id, timestamp);
CREATE INDEX idx_file_activity_path ON file_activity(path, timestamp);

CREATE TABLE git_events (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id    TEXT NOT NULL REFERENCES sessions(session_id),
    command       TEXT NOT NULL,
    subcommand    TEXT NOT NULL,
    branch        TEXT,
    target_branch TEXT,
    timestamp     INTEGER NOT NULL,
    result        TEXT
);

CREATE INDEX idx_git_events_session ON git_events(session_id, timestamp);
CREATE INDEX idx_git_events_branch ON git_events(branch, timestamp);

CREATE TABLE messages (
    message_id    TEXT PRIMARY KEY,
    thread_id     TEXT NOT NULL,
    from_session  TEXT NOT NULL REFERENCES sessions(session_id),
    to_session    TEXT NOT NULL,
    body          TEXT NOT NULL,
    state         TEXT NOT NULL DEFAULT 'queued',
    created_at    INTEGER NOT NULL,
    delivered_at  INTEGER
);

CREATE INDEX idx_messages_to ON messages(to_session, state);
CREATE INDEX idx_messages_thread ON messages(thread_id, created_at);
```

### 10.2 Timestamps

All timestamps are stored as integer unix timestamps (seconds since epoch). Integer comparison is faster than string comparison for the time-range queries Pane runs constantly ("file activity in the last 15 minutes", "session seen in the last 4 hours"). Human-readable formatting is handled in Go at display time.

### 10.3 Retention

File activity older than 72 hours is pruned on daemon startup and periodically during operation. This covers overnight and weekend gaps so context survives common work interruptions. Git events and messages are retained for 7 days. Sessions in `closed` status are archived after 7 days.

### 10.4 Concurrency

The daemon serializes all database writes through a single SQLite connection in WAL mode. Multiple concurrent CLI requests are handled by the daemon's goroutine pool, but database writes are funneled through a single writer goroutine to avoid SQLite lock contention. This is appropriate for V1's expected write volume (tens of writes per second at most). Reads can happen concurrently via separate read-only connections.

---

## 11. Daemon lifecycle

### 11.1 Starting

```bash
pane daemon start
```

The daemon:

1. creates `~/.pane/` directory if it doesn't exist
2. opens or creates `~/.pane/pane.db`
3. runs migrations
4. starts listening on `~/.pane/pane.sock`
5. starts the file watcher for registered workspaces
6. writes its PID to `~/.pane/pane.pid`

### 11.2 Stopping

```bash
pane daemon stop
```

Sends SIGTERM to the daemon PID. The daemon:

1. stops accepting new connections
2. flushes pending writes to SQLite
3. closes the socket
4. removes the PID file
5. exits

### 11.3 Auto-start

`pane` CLI checks for a running daemon before each request. If the socket is missing or unresponsive, `pane` can optionally start the daemon automatically in the background.

To prevent race conditions when multiple CLI invocations detect a missing daemon simultaneously, startup uses a file lock at `~/.pane/pane.lock`. The first process acquires the lock, starts the daemon, and releases it. Subsequent processes wait on the lock, then connect to the now-running daemon.

### 11.4 Health

```bash
pane daemon health
```

Returns daemon uptime, active session count, database size, and watcher status.

---

## 12. Shell integration

### 12.1 Minimal setup

Add to `.bashrc` / `.zshrc`:

```bash
# Option A: alias git
alias git='pane git'

# Option B: PATH shim (transparent to agents)
export PATH="$HOME/.pane/shims:$PATH"

# Auto-register session on shell start
eval "$(pane shell-init)"
```

`pane shell-init` outputs a shell hook that:

1. runs `pane init` to register the session
2. sets up `precmd` / `PROMPT_COMMAND` to heartbeat the session and update branch/cwd
3. prints the session-start summary

### 12.2 Zellij-specific

When `$ZELLIJ_PANE_ID` is available, Pane uses it as the pane identifier. This gives stable session identity across shell restarts within the same Zellij pane.

---

## 13. Failure modes

### 13.1 Daemon not running

`pane` attempts to connect to the socket. If it fails:

- for `pane git`: execute the real git command directly with a stderr note: `[Pane] daemon not running, proceeding without coordination`
- for `pane ask/inbox/reply`: print an error and exit

Pane must never block normal development work.

### 13.2 Daemon crashes

The daemon writes all state to SQLite synchronously. On restart, all session and message state is recovered. File watcher state is rebuilt from the filesystem.

### 13.3 Database corruption

If SQLite fails to open, the daemon moves the corrupt file to `pane.db.corrupt.<timestamp>` and creates a fresh database. Sessions will need to re-register, but no work is lost — only coordination history.

### 13.4 Attribution failure

If the file watcher cannot attribute a write to a session, the event is recorded with `attribution = 'low'`. Low-attribution events are stored for debugging but do not trigger warnings to other sessions.

---

## 14. Directory layout

```
~/.pane/
├── pane.sock           # Unix domain socket
├── pane.pid            # daemon PID file
├── pane.lock           # file lock for daemon auto-start
├── pane.db             # SQLite database
├── pane.db-wal         # WAL file
├── pane.db-shm         # shared memory file
├── shims/
│   └── git             # optional PATH shim
└── logs/
    └── pane.log        # daemon log (optional, for debugging)
```

---

## 15. Project structure

```
pane/
├── cmd/
│   └── pane/
│       └── main.go           # single binary entry point
├── internal/
│   ├── cli/
│   │   ├── root.go           # root command, subcommand routing
│   │   ├── daemon.go         # daemon start/stop/health subcommands
│   │   ├── git.go            # pane git subcommand
│   │   ├── session.go        # init, status, intent subcommands
│   │   └── messages.go       # ask, inbox, reply subcommands
│   ├── daemon/
│   │   ├── daemon.go         # daemon server, socket listener
│   │   └── handler.go        # request routing and dispatch
│   ├── session/
│   │   ├── session.go        # session types and lifecycle
│   │   └── manager.go        # create, resume, heartbeat, close
│   ├── gitguard/
│   │   ├── preflight.go      # git preflight evaluation
│   │   └── parser.go         # git subcommand parsing
│   ├── activity/
│   │   ├── watcher.go        # filesystem watcher
│   │   ├── attribution.go    # process tree → session mapping
│   │   └── workingset.go     # working set derivation, overlap
│   ├── messages/
│   │   ├── message.go        # message types
│   │   └── router.go         # send, inbox, reply, thread tracking
│   ├── summary/
│   │   ├── startup.go        # session-start summary generation
│   │   └── preflight.go      # git preflight warning generation
│   ├── store/
│   │   ├── db.go             # SQLite connection, migrations
│   │   ├── sessions.go       # session CRUD
│   │   ├── file_activity.go  # file activity CRUD
│   │   ├── git_events.go     # git event CRUD
│   │   └── messages.go       # message CRUD
│   └── protocol/
│       ├── request.go        # request types
│       └── response.go       # response types
├── docs/
│   ├── 80-20-overview.md
│   └── architecture.md       # this document
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── .gitignore
```

---

## 16. Build and install

```bash
# build the binary
make build

# install to ~/.pane/bin/
make install

# run tests
make test
```

The Makefile compiles `cmd/pane` into `bin/pane`.

`make install` copies the binary to `~/.pane/bin/` and optionally creates the git shim at `~/.pane/shims/git`.

---

## 17. Constraints and non-goals for V1

### Constraints

- Go only. No Rust in V1. The semantic analysis engine is a future addition.
- SQLite only. No external database dependencies.
- Local only. No network communication.
- macOS and Linux only.
- No agent modification required. Pane works with any agent that runs shell commands.

### Non-goals for V1

- Symbol-level dependency graphs
- Tree-sitter or any AST parsing
- Automatic natural-language reply capture
- Shell interception beyond git
- Worker/child session hierarchies
- Learned workflow models
- UI or dashboard
- Remote/cloud deployment
