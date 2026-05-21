# Pane Agent Instructions

Pane is shared local memory for agent work in this workspace.

If you are an agent operating in this repo, use Pane as part of your normal loop. The human should not have to manually relay context between sessions.

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

## What to pay attention to

- Keep `pane intent` current.
- Treat `pane board` as the shared awareness board.
- Treat `pane summary` as startup/resume context.
- Use `pane continue <session-id>` when inheriting work from a previous session.
- Use `pane state` for small namespaced JSON facts that should persist locally.
- Use `pane ask` / `pane reply` for coordination.
- Do not assume the human knows what other panes are doing.
