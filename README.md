# Pane

Pane is shared local memory for concurrent coding agents.

When you run several agents at once, the hidden tax is coordination. One pane is refactoring auth, another is writing tests, a third is about to rebase, and you are the one remembering who touched what, which assumptions changed, and which git command might stomp on someone else's work.

Pane moves that coordination layer out of your head and into the workspace.

It gives agent sessions a shared awareness board: who is active, what they are trying to do, what files they have touched recently, what questions are unresolved, and which operations may affect other sessions. Agents read from and write to that board themselves, so the human is no longer the message bus between panes.

Pane is not an orchestrator. It does not assign tasks or boss agents around. It sits below them, at the local shell/workspace layer, and gives them enough shared context to collaborate without requiring a provider-specific integration.

If an agent can run shell commands, it can participate in Pane.

## Why this matters

Multi-agent coding currently feels powerful but fragile:

- agents lose context when restarted
- agents do not know what other panes are doing
- humans manually relay findings between sessions
- risky operations happen without shared-state awareness
- coordination history disappears into terminal scrollback

Pane's long-term vision is to become the local shared-memory layer for that workflow: a fast, private, provider-agnostic surface where coding agents coordinate through the filesystem, terminal pane identity, command interception, file activity, and explicit messages.

## Core use cases

Pane is meant to solve these workflows first:

- **Agent restart continuity** — a new agent can inherit useful context instead of starting cold.
- **Cross-pane handoff** — work can move from one terminal pane or agent to another with explicit lineage.
- **Concurrent agent awareness** — agents can see nearby work, current intents, recent files, and open questions.
- **Human handoff relief** — the human should not have to manually summarize and relay every bit of shared context.
- **Workspace memory over terminal scrollback** — important context becomes queryable local memory instead of disappearing into logs.
- **Safer high-risk operations** — shared awareness can warn before git operations that may disrupt another session.
- **Specialized agent memory** — agents can store compact namespaced JSON facts without inventing per-tool caches.
- **Provider-agnostic collaboration** — any agent that can run shell commands can participate.

See [`USE_CASES.md`](USE_CASES.md) for the fuller use-case narrative.

## What Pane gives agents

Pane gives each session a durable identity tied to the terminal pane, not the agent process. If one agent exits and another starts in the same pane, the new agent can inherit the pane's context: current intent, recent activity, messages, and nearby work from other sessions.

The core surface is the shared awareness board. A Pane-aware agent can ask:

- who else is active in this workspace?
- what is each session currently trying to do?
- which files or directories were touched recently?
- where might my work overlap with another session?
- are there unread questions or unresolved coordination threads?
- is this operation risky given what other sessions are doing?

The human can inspect the board, but should not have to maintain it. Agents are expected to update their own state as they work.

Lifecycle behavior: Pane persists sessions in SQLite across daemon restarts. Restarting the daemon does not delete old sessions. `pane board` hides closed sessions and first-pass stale sessions by default; use `pane close` to close the current session and `pane sessions prune` to close stale active/idle sessions in the workspace.

## Current status

**V1 is complete.** Pane is a working local coordination layer with daemon-backed sessions, board, summary, messaging, file activity, git guardrails, shell integration, continuity, agent state, and session lifecycle cleanup. Dogfooded and verified 2026-05-21.

V2 work (overlap detection, richer preflight, daemon hardening) is scoped in [`ROADMAP.md`](ROADMAP.md).

## V1 focus

V1 is the 80/20 foundation. It does not try to understand every symbol or perfectly model the codebase. It provides the pieces that remove the most human glue work:

- pane-based session identity
- shared awareness board
- current intent/status updates from agents
- file-level working set awareness
- session-start summaries
- explicit inter-session messaging
- git preflight checks as an early guardrail

Git is a feature, not the product. Pane's product is shared workspace awareness. Git interception is simply one high-leverage place to apply that awareness before damage happens.

## Agent operating pattern

A Pane-aware agent should use Pane commands during its normal loop:

```bash
pane init
pane heartbeat
pane board
pane close
pane summary
pane history --since 24h
pane continue <session-id>
pane intent "working on auth middleware"
pane inbox
pane ask <session-id-or-short-id> "are you still touching auth/session.ts?"
pane reply <message-id> "done with that file"
pane state set agent.notes '{"handoff":"tests need review"}'
pane state get agent.notes
pane git status
```

Agents should update `pane intent` whenever they switch tasks. The board is only useful if each participating session writes its own current state.

## Project docs

- [`ROADMAP.md`](ROADMAP.md) — **start here for what's next**: V2, V3, and Done plan
- [`ARCHITECTURE.md`](ARCHITECTURE.md) — agent-readable system architecture and design rules
- [`PROGRESS.md`](PROGRESS.md) — committed phase plan, current status, and V1 dogfood results
- [`USE_CASES.md`](USE_CASES.md) — concrete problems and workflows Pane is meant to solve
- [`V1_READY.md`](V1_READY.md) — definition of V1 (achieved)
- [`REFRAMING.md`](REFRAMING.md) — long-term reframe from coordination tool to environment-as-product
- [`docs/architecture.md`](docs/architecture.md) — detailed V1 technical architecture
- [`docs/80-20-overview.md`](docs/80-20-overview.md) — V1 product scope

## Current repo status

**V1 is done.** All phases through session lifecycle cleanup are implemented and dogfooded. The V2/V3/Done roadmap is defined in [`ROADMAP.md`](ROADMAP.md).

The repository contains:

- CLI entry point
- SQLite persistence foundation
- pane/TTY session identity
- daemon-backed `pane init`, `pane heartbeat`, `pane status`, and `pane intent`
- daemon-backed `pane board` with active session visibility, short session IDs, and coordination indicators
- daemon-backed `pane summary` with session-specific startup context, unread messages, recent files, and continuity history
- daemon-backed `pane ask`, `pane inbox`, and `pane reply` messaging
- first-pass daemon-observed file activity
- first-pass sequential continuity with `pane continue` and `pane history`
- first-pass generic agent state with `pane state set|get|list|delete`
- first-pass `pane git` passthrough, preflight, and event recording
- Unix socket daemon foundation with first-pass PID/status lifecycle metadata
- protocol codec and request types
- initial tests

The current focus is V2: overlap detection, richer git preflight, board/summary signal quality, daemon hardening, and targeting ergonomics. See [`ROADMAP.md`](ROADMAP.md) for the full guided plan.

## Project shape

- `cmd/pane` — single CLI binary
- `internal/session` — pane/session identity and lifecycle
- `internal/board` — shared awareness board model and rendering
- `internal/activity` — file activity tracking and overlap detection
- `internal/messages` — ask/inbox/reply flow
- `internal/gitguard` — git preflight logic
- `internal/store` — SQLite persistence
- `internal/daemon` — local daemon and socket server
- `internal/protocol` — CLI/daemon protocol types
- `docs/` — product and technical notes

## Development

```bash
make test
make build
./bin/pane help
./bin/pane daemon start
./bin/pane daemon status
```

## Shell integration

Pane can print a shell hook that starts the daemon, registers the pane session, and runs `pane heartbeat` on each prompt:

```bash
eval "$(/path/to/pane shell-init)"
```

For transparent git guardrails, install the git shim and prepend it to PATH:

```bash
pane shims install
export PATH="$HOME/.pane/shims:$PATH"
```

Agents should read [`AGENTS.md`](AGENTS.md) for operating instructions.
