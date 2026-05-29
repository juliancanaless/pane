# Pane Agent Instructions

Pane is shared local memory for agent work in this workspace.

If you are an agent operating in this repo, use Pane as part of your normal loop. The human should not have to manually relay context between sessions.

> **Cross-agent note:** This file is the canonical operating contract for ALL coding agents — Claude Code, Codex, Cursor, Copilot, Pi, Windsurf, Aider, or anything else. Provider-specific discovery files (CLAUDE.md, .cursorrules, etc.) exist in this repo solely to point back here. Pane works at the shell level; if you can run commands, you can participate.

## Operating contract

Your job is to keep Pane state useful while you work. The human can inspect Pane, but should not have to maintain it for you.

1. initialize/resume your session
2. read summary/board before acting
3. set intent before meaningful changes
4. update intent when you switch tasks
5. check/respond to messages
6. ask/reply through Pane instead of asking the human to relay routine coordination
7. use Pane git guardrails for git commands
8. leave durable handoff/state when useful
9. close your session when done

## Startup

At the beginning of work, run:

```bash
./bin/pane init
./bin/pane heartbeat
./bin/pane summary
./bin/pane board
```

If you are taking over from a known prior session, link the handoff before proceeding:

```bash
./bin/pane continue <session-id-or-short-id>
./bin/pane summary
```

If `./bin/pane` does not exist, run:

```bash
make build
```

If Pane says the daemon is not running, ask for or start a daemon in a separate terminal:

```bash
./bin/pane daemon start
```

## During work

Set your current task before making meaningful changes:

```bash
./bin/pane intent "short description of what this session is doing"
```

Update intent whenever you switch tasks. Run `./bin/pane heartbeat` if you change directories or branches and want Pane state refreshed before the next shell prompt.

Check shared state before acting in an area another session may be touching:

```bash
./bin/pane board
./bin/pane summary
./bin/pane history --since 24h
```

## Messaging

Ask other sessions directly instead of routing through the human:

```bash
./bin/pane ask <session-id-or-short-id> "question"
./bin/pane inbox
./bin/pane reply <message-id> "answer"
```

To coordinate with a session in another repository or workspace, list every
session on the machine, then message one by its full session id:

```bash
./bin/pane board --global                              # all sessions, all workspaces
./bin/pane ask --global <full-session-id> "question"   # reach a session anywhere
```

`--global` requires the full session id (shown on the global board); names and
short ids only resolve within your own workspace to avoid cross-machine
collisions. Replies and inbox work the same regardless of which workspace the
peer is in.

## Persistent state

Use namespaced JSON state for compact facts that should survive the current chat/session but do not belong in source files:

```bash
./bin/pane state set agent.notes '{"handoff":"tests need review"}'
./bin/pane state get agent.notes
./bin/pane state list agent.
./bin/pane state delete agent.notes
```

## Git

Use Pane's git wrapper for git commands:

```bash
./bin/pane git status
./bin/pane git commit --dry-run
```

If shell shims are installed and `git` already routes through Pane, normal `git` commands are fine.

## Minimum normal loop

Use this loop whenever you start or switch meaningful work:

```bash
./bin/pane heartbeat
./bin/pane summary
./bin/pane board
./bin/pane inbox
./bin/pane intent "what I am doing now"
```

Before risky git operations, use:

```bash
./bin/pane git status
./bin/pane git <args...>
```

Before ending or handing off, use:

```bash
./bin/pane state set agent.handoff '{"summary":"what changed","next":"what to do next"}'
./bin/pane history --since 24h
./bin/pane close
```

If the board appears to show stale sessions, run:

```bash
./bin/pane sessions prune
./bin/pane board
```

## What to pay attention to

- Keep `pane intent` current.
- Treat `pane board` as the shared awareness board.
- Treat `pane summary` as startup/resume context.
- Use `pane continue <session-id>` when inheriting work from a previous session.
- Use `pane state` for small namespaced JSON facts that should persist locally.
- Use `pane ask` / `pane reply` for coordination.
- Do not assume the human knows what other panes are doing.
- Do not rely on the human to update Pane state, summarize your work, or relay routine cross-agent messages.
- Sessions persist across daemon restarts. `pane board` hides stale/closed sessions by default, while `pane history` keeps durable past context.
- Use `pane close` before ending a pane's work when possible.
