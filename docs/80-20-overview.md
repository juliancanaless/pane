# Pane 80/20 Overview

Pane's highest-leverage v1 is:

> a local daemon that knows which pane is working on which files, warns before risky git operations, and lets sessions message each other.

## Core use cases

Pane's features should stay grounded in the workflows they support:

- **Agent restart continuity**: a replacement agent can recover what was happening without a human-written handoff.
- **Cross-pane handoff**: one pane or agent can explicitly continue another session's work.
- **Concurrent agent awareness**: agents can see who else is active, what they are doing, and where work may overlap.
- **Human handoff relief**: the human is not the message bus or shared-memory layer between agents.
- **Workspace memory over terminal scrollback**: sessions, intents, messages, recent files, and lineage are queryable.
- **Safer high-risk operations**: git guardrails use shared state before disruptive operations happen.
- **Specialized agent memory**: agents can store compact namespaced JSON facts without inventing per-tool caches.
- **Provider-agnostic collaboration**: coordination works at the shell/workspace layer for any agent.

The longer narrative lives in [`../USE_CASES.md`](../USE_CASES.md).

## V1 capabilities

- stable session identity tied to pane/TTY
- shared awareness board for active sessions
- file-level working set tracking
- session-start summaries
- explicit async messaging
- git-only interception via `pane git`
- targeted git preflight warnings
- generic workspace-scoped state via `pane state`

## Shared awareness board

Pane needs a shared board because humans cannot keep up with manually tracking every agent's current task, touched files, open questions, and risky operations.

The board is not a project-management board and not a lock map. It is the local coordination surface agents read from and write to while working.

At minimum, the board should show:

- active sessions in the workspace
- each session's current intent
- each session's branch/cwd when available
- recent files and hot directories per session
- overlap between sessions
- unread messages and open coordination threads
- recent relevant git or guardrail events

Agents should update this board themselves. In practice that means a Pane-aware agent should run commands such as:

- `pane init` when starting or resuming
- `pane summary` before beginning work
- `pane intent "..."` when starting or changing tasks
- `pane inbox` to check coordination messages
- `pane ask` / `pane reply` to route context without the human
- `pane git ...` for guarded git operations

The human can inspect the board, but the human should not be responsible for maintaining it.

## Core user problems

1. agents restart or get replaced and lose the thread
2. work moves between panes without an explicit handoff trail
3. several agents work in the same area without contextual awareness of each other
4. the human manually relays context between agents
5. useful workspace history disappears into terminal scrollback
6. risky git actions can stomp on another session's work
