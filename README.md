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

## V1 focus

The first version is intentionally practical. It does not try to understand every symbol or perfectly model the codebase. It starts with the pieces that remove the most human glue work:

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
pane board
pane summary
pane intent "working on auth middleware"
pane inbox
pane ask <session-id> "are you still touching auth/session.ts?"
pane reply <message-id> "done with that file"
pane git status
```

Agents should update `pane intent` whenever they switch tasks. The board is only useful if each participating session writes its own current state.

## Project docs

- [`ARCHITECTURE.md`](ARCHITECTURE.md) — agent-readable system architecture and design rules
- [`PROGRESS.md`](PROGRESS.md) — committed phase plan, current status, and testing/dogfooding checklist
- [`REFRAMING.md`](REFRAMING.md) — long-term reframe from coordination tool to environment-as-product
- [`docs/architecture.md`](docs/architecture.md) — detailed V1 technical architecture
- [`docs/80-20-overview.md`](docs/80-20-overview.md) — V1 product scope

## Current repo status

This repository currently contains the early Go scaffold and first working slices:

- CLI entry point
- SQLite persistence foundation
- pane/TTY session identity
- daemon-backed `pane init`, `pane status`, and `pane intent`
- daemon-backed `pane board` with active session visibility
- daemon-backed `pane summary` for session-specific startup context
- daemon-backed `pane ask`, `pane inbox`, and `pane reply` messaging
- Unix socket daemon foundation
- protocol codec and request types
- initial tests

The next major work is to surface message state in board/summary, then add file activity and git preflight behavior. See [`PROGRESS.md`](PROGRESS.md) for the current phase plan.

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
```
