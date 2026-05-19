# Pane 80/20 Overview

Pane's highest-leverage v1 is:

> a local daemon that knows which pane is working on which files, warns before risky git operations, and lets sessions message each other.

## V1 capabilities

- stable session identity tied to pane/TTY
- shared awareness board for active sessions
- file-level working set tracking
- session-start summaries
- explicit async messaging
- git-only interception via `pane git`
- targeted git preflight warnings

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

1. several agents are working in the same area of the codebase on different tasks, and that creates uneasiness because they lack contextual awareness of what the others are doing
2. you don't know a git action is about to stomp on someone else's work
3. you manually relay context between agents
