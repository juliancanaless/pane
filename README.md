# Pane

**Shared local memory for AI coding agents.**

Pane gives Claude Code, Codex, Cursor-style terminal agents, and other AI coding agents a local coordination layer: sessions, intents, messages, file activity, worktree awareness, agent memory, work logs, and git guardrails.

The core idea is simple: **agents should keep the coordination surface up to date themselves so the human does not have to be the message bus.**

When several agents run at once, the hidden tax is coordination. One pane is refactoring auth, another is writing tests, a third is about to rebase, and the human is usually the one remembering who touched what, which assumptions changed, and which git command might stomp on someone else's work. Pane moves that shared memory into the local workspace.

Pane is not an orchestrator. It does not assign tasks, schedule agents, or control providers. It sits below them at the shell/workspace layer. If an agent can run shell commands, it can participate.

## Install

Pane v0.1.5 can be installed with the Homebrew tap or built from source. Homebrew binary assets are available for Intel and Apple Silicon Macs. See [`INSTALL.md`](INSTALL.md).

```bash
brew tap juliancanaless/pane
brew install pane
pane setup
pane doctor
pane docs quickstart
pane docs agents
```

## Who is this for?

Pane is for developers running multiple AI coding agents at once:

- Claude Code in one terminal pane
- Codex or another CLI coding agent in another pane
- Cursor or editor-integrated agents changing nearby files
- separate Git worktrees for isolated parallel tasks
- humans who do not want to manually relay context between agents

Pane is local-first shared memory and coordination for terminal-based coding agents.

## The agent contract

Pane only works well if agents use it as part of their normal loop. The human should not have to keep Pane updated manually.

A Pane-aware agent is expected to:

1. initialize or resume its session with `pane init`
2. read `pane summary`, `pane board`, and `pane inbox` before acting
3. set `pane intent "..."` before meaningful changes
4. update intent whenever the task changes
5. use `pane ask` / `pane reply` to coordinate with other sessions instead of routing through the human
6. use `pane git ...` for high-risk git commands
7. store compact handoff or memory facts with `pane state`
8. close its session with `pane close` when finished

The board is useful only when participating agents write their own state. Pane is designed so agents can maintain this state themselves.

Agents can also discover the same guidance from the installed binary:

```bash
pane docs quickstart
pane docs agents
pane docs faq
```

See [`AGENTS.md`](AGENTS.md) and [`docs/for-agents.md`](docs/for-agents.md) for the full operating contract agents should follow.

## Core use cases

- **Agent restart continuity** — a new agent can inherit useful context instead of starting cold.
- **Cross-pane handoff** — work can move from one terminal pane or agent to another with explicit lineage.
- **Concurrent agent awareness** — agents can see nearby work, current intents, recent files, and open questions.
- **Human handoff relief** — the human should not have to summarize and relay every bit of shared context.
- **Workspace memory over terminal scrollback** — important context becomes queryable local memory instead of disappearing into logs.
- **Safer high-risk operations** — shared awareness can warn before git operations that may disrupt another session.
- **Specialized agent memory** — agents can store compact namespaced JSON facts without inventing per-tool caches.
- **Provider-agnostic collaboration** — any agent that can run shell commands can participate.

## What Pane provides

### Shared awareness board

```bash
pane board
pane board --repo
```

Shows active sessions, current intents, cwd/branch, recent files, overlaps, coordination indicators, and recent git events. `--repo` aggregates sibling Git worktrees that belong to the same repository.

### Startup / resume context

```bash
pane summary
```

Shows the current session, peer sessions, unread messages, recent activity, continuity context, overlaps, semantic warnings, and selected `summary.*` state.

### Agent-maintained intent

```bash
pane intent "working on auth middleware"
```

Agents should update this when they switch tasks. This is the cheapest way to keep other agents and the human oriented.

### Messaging

```bash
pane ask <session-id-or-name> "are you still touching auth/session.ts?"
pane inbox
pane reply <message-id> "done with that file"
```

Agents can coordinate directly without the human copying messages between panes.

### Git guardrails

```bash
pane git status
pane git rebase main
pane git push --force-with-lease
```

Pane records git activity and can warn when a command may disrupt another active session.

### Worktree-aware scope

```bash
pane board --repo
pane history --repo --lineage
```

Each worktree keeps isolated cwd/file activity, while repo scope aggregates related sessions by Git common-dir identity.

### Worker/child lineage

```bash
pane spawn <command> [args...]
pane history --lineage
```

Child commands register as child sessions and appear in board/history lineage.

### Agent memory

```bash
pane state set agent.notes '{"handoff":"tests need review"}'
pane state set summary.note '{"text":"auth token shape changed"}'
pane state namespaces
pane state list agent.
pane state set --global neon.memory '{"prefers":"workspace summaries"}'
```

Use dotted namespaces for compact local memory. `summary.*` keys appear in startup summaries.

### Work logs

```bash
pane history --since 1w --format work-log
```

Produces a compact report with sessions, durations, file counts, git operation counts, and intents.

## Common agent loop

```bash
pane init
pane heartbeat
pane summary
pane board
pane inbox
pane intent "what I am doing now"
```

Before risky work:

```bash
pane board --repo
pane git status
```

Before ending or handing off:

```bash
pane state set agent.handoff '{"summary":"what changed","next":"what to do next"}'
pane history --since 24h --lineage
pane close
```

## Storage and retention

Pane stores local state in SQLite under `~/.pane` by default. It is intentionally local-first and private.

Current behavior:

- sessions, messages, file activity, git events, agent state, and analysis data are persisted locally
- closed/stale sessions are hidden from the default board but remain queryable in history
- activity views use decay windows so old file activity is summarized or ignored in output
- there is not yet an aggressive automatic pruning/compaction policy for the SQLite database

In normal use the database should stay small, but long-running/high-volume installations may eventually need explicit retention controls such as `pane vacuum`, configurable history windows, or automatic pruning. That is a future hardening area rather than a v0.1.1 feature.

## Current status

Pane is in an early shareable release. The main feature scaffolding is implemented and CI passes on macOS and Linux.

Implemented today:

- daemon-backed sessions, board, summary, messaging, history, and state
- file activity, overlap warnings, and git preflight guardrails
- Rust/tree-sitter analyzer helper for symbols and dependency edges
- first-pass semantic overlap warnings based on dependency data
- temporal activity decay
- Git worktree-aware repo scope
- worker/child sessions with `pane spawn`
- lineage view with `pane history --lineage`
- namespaced and global agent state
- work-log history output
- `pane setup`, `pane doctor`, Homebrew tap, and macOS Intel/Apple Silicon release assets
- macOS/Linux CI with daemon smoke tests

Known limitations:

- semantic overlap is first-pass import/package-level, not full symbol-reference/signature-aware analysis
- Linux binary release assets are not published yet, though source CI passes on Linux
- storage retention is basic; old data is hidden/summarized in views but not aggressively pruned
- Pane needs more multi-day dogfood with real teams and real concurrent agent workflows

## Project docs

- [`INSTALL.md`](INSTALL.md) — installation and setup
- [`FAQ.md`](FAQ.md) — common user questions and edge cases
- [`AGENTS.md`](AGENTS.md) — operating contract for agents in this repo
- [`docs/for-agents.md`](docs/for-agents.md) — command guide for AI coding agents using Pane
- [`docs/demo.md`](docs/demo.md) — text demo transcript for humans and agents
- [`USE_CASES.md`](USE_CASES.md) — concrete problems and workflows Pane is meant to solve
- [`ARCHITECTURE.md`](ARCHITECTURE.md) — agent-readable system architecture
- [`PROGRESS.md`](PROGRESS.md) — detailed implementation history and current gaps
- [`ROADMAP.md`](ROADMAP.md) — development roadmap and historical phase plan

## Why Go and Rust?

Pane is basically a small local distributed system beneath the agents, which made Go feel natural for the daemon/CLI coordination layer. Rust is used for `pane-analyze` because parsing and semantic analysis benefit from its performance, safety, and tree-sitter ecosystem.

## Development

```bash
make test
make build
make restart # stop the current dev daemon, rebuild, and start it in the background
./bin/pane help
./bin/pane setup
./bin/pane setup --no-shell --no-daemon # install without editing shell rc or starting daemon
./bin/pane doctor
./bin/pane daemon start
./bin/pane daemon status
```

`make install` and `pane setup` install both `pane` and the `pane-analyze` helper used for semantic analysis. The Homebrew tap is `juliancanaless/pane`. CI runs on macOS and Linux with Go/Rust tests, build, and a daemon smoke test.

## Shell integration

Pane can print a shell hook that starts the daemon, registers the pane session, and runs `pane heartbeat` on each prompt:

```bash
eval "$(pane shell-init)"
```

For transparent git guardrails, install the git shim and prepend it to PATH:

```bash
pane shims install
export PATH="$HOME/.pane/shims:$PATH"
```
