# Pane

Pane is a local coordination layer for multi-agent development workflows.

## Initial focus

This repo starts with the 80/20 approach:

- pane-based session identity
- a shared awareness board showing what active sessions are doing
- file-level working set awareness
- session-start summaries
- explicit inter-session messaging
- git preflight interception as an early guardrail

## Shared awareness board

Pane's core surface is a local shared awareness board. Agents use it to see:

- active sessions in the workspace
- each session's current intent
- recent files and directories each session has touched
- branch/cwd/session status when available
- unresolved coordination messages
- recent warnings or risky operations

Humans should not have to keep this board current manually. Agents are expected to update their own state as they work.

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

Agents should update `pane intent` when they switch tasks. The shared board is only useful if each participating session writes its own current state.

## Proposed shape

- `cmd/pane` — single CLI binary (daemon, git interception, messaging, session management)
- `internal/session` — pane/session identity and lifecycle
- `internal/gitguard` — git preflight logic
- `internal/activity` — file activity tracking and overlap detection
- `internal/board` — shared awareness board model and rendering
- `internal/messages` — ask/inbox/reply flow
- `internal/store` — SQLite persistence
- `docs/` — product and technical notes

## Next steps

1. define the v1 repo structure
2. write the 80/20 technical spec
3. implement session registry
4. implement `pane git ...` preflight flow
