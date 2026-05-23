# Pane Demo Transcript

This is a text-first demo for humans and AI agents scanning the repository.

## Install

```bash
brew tap juliancanaless/pane
brew install pane
pane setup
pane doctor
```

## Start a session

```bash
pane init
pane intent "refactor auth middleware"
pane summary
```

Pane records a durable session tied to the terminal pane, not a single process.

## See peer sessions

```bash
pane board
```

Example shape:

```text
[Pane] Workspace board
Workspace: /repo
Sessions: 2

session-a (short: a) — active — main
  Intent: refactor auth middleware
  CWD: auth
  Recent files: auth/session.ts, auth/token.ts

session-b (short: b) — active — tests
  Intent: write auth tests
  CWD: tests
```

## Coordinate explicitly

```bash
pane ask b "Are you still editing auth/session.ts?"
pane inbox
pane reply msg-123 "Done with that file now."
```

## Use git guardrails

```bash
pane git rebase main
```

Pane can warn if another active session is on the same branch, touching overlapping files, or depending on recently changed files.

## Use worktrees without losing awareness

```bash
pane board --repo
pane history --repo --lineage
```

Each Git worktree keeps separate cwd/file activity, while repo scope aggregates shared repository awareness.

## Spawn a child worker

```bash
pane spawn sh -c 'pane intent "write auth tests"; make test'
pane history --lineage
```

Pane records the child session and links it to the parent.

## Store agent memory

```bash
pane state set agent.notes '{"handoff":"auth tests need review"}'
pane state set summary.note '{"text":"auth token shape changed"}'
pane state namespaces
pane summary
```

`summary.*` state appears in startup summaries for future agents.

## Generate a work log

```bash
pane history --since 1w --format work-log
```

Example output shape:

```text
[Pane] Work log
Workspace: /repo
Sessions: 4
Files touched: 18
Git operations: 6

- auth-refactor — closed — main
  Intent: refactor auth middleware
  Duration: 2h13m0s
  Files touched: 9
  Git operations: 2
```
