# Pane for AI Coding Agents

If you are an AI coding agent operating in a terminal, use Pane as local shared memory for the workspace.

Pane is provider-agnostic. It works for Claude Code, Codex, Cursor-style terminal workflows, custom shell agents, and any coding agent that can run commands.

## What Pane gives you

- A durable session identity for the current terminal pane
- A shared awareness board of active peer sessions
- Current intent/status for each agent
- Recent file activity and overlap warnings
- Async coordination messages with `ask`, `inbox`, and `reply`
- Git preflight guardrails before risky operations
- Worktree-aware repository scope with `--repo`
- Worker/child session tracking with `pane spawn`
- Persistent namespaced agent memory with `pane state`
- Work-log history for handoffs and weekly reports

## Recommended startup loop

Run these near the start of a task:

```bash
pane init
pane heartbeat
pane summary
pane board
pane inbox
```

Then set your intent:

```bash
pane intent "brief description of current task"
```

Update intent whenever you materially change tasks.

## Before editing or running risky commands

Check nearby work:

```bash
pane board
pane board --repo
pane summary
```

If another session is touching related files, coordinate:

```bash
pane ask <session-name-or-id> "Are you still working in auth/?"
pane inbox
pane reply <message-id> "I am done with auth/session.ts"
```

Use Pane's git wrapper for high-risk operations:

```bash
pane git status
pane git rebase main
pane git push --force-with-lease
```

## Child/worker tasks

Spawn a child command so Pane can track lineage:

```bash
pane spawn <command> [args...]
```

Inspect lineage:

```bash
pane history --lineage
```

## Agent memory conventions

Use dotted namespaces. Examples:

```bash
pane state set agent.notes '{"handoff":"tests need review"}'
pane state set summary.note '{"text":"remember to update migration docs"}'
pane state list agent.
pane state namespaces
```

Use `summary.*` for compact state that should appear in `pane summary` for future agents.

Use global state only for cross-workspace facts:

```bash
pane state set --global neon.memory '{"prefers":"workspace summaries"}'
pane state get --global neon.memory
```

## Handoff commands

At the end of work, useful commands are:

```bash
pane intent "done: short summary of completed work"
pane history --since 24h --lineage
pane history --since 1w --format work-log
pane close
```

## Important behavior

- Pane is local-first. State lives in SQLite under `~/.pane` by default.
- Pane is not an orchestrator. It does not assign tasks or control agents.
- The board is only useful if agents keep their own intent current.
- Worktree-local context remains isolated, while `--repo` aggregates related Git worktrees.
