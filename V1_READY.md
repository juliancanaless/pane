# Pane V1 Ready Definition

Pane is "good enough to use" when agents can rely on it as the shared local memory layer without the human constantly correcting context.

This is not the same as feature-complete forever. It is the bar for serious daily dogfooding.

## V1 ready means

### 1. Agents know how to participate

An agent can read `AGENTS.md` and know the normal loop:

```bash
./bin/pane init
./bin/pane summary
./bin/pane board
./bin/pane intent "what I am doing"
./bin/pane inbox
./bin/pane git status
```

The agent knows when to use:

- `pane continue <session-id-or-short-id>` when inheriting work
- `pane ask` / `pane reply` for coordination
- `pane state` for compact persistent JSON facts
- `pane heartbeat` when cwd/branch/session freshness matters

### 2. The board feels trustworthy

`pane board` should show the sessions that matter now, not every durable session ever remembered.

V1 ready requires:

- clear active / idle / stale / closed semantics
- a way to close the current session
- stale sessions hidden from the default board
- old sessions still visible through history

### 3. Startup context is useful

A new or restarted agent can run:

```bash
./bin/pane summary
./bin/pane history --since 24h
```

and understand:

- what this pane/session is
- what recent sessions were doing
- whether this work continues a prior session
- whether unread messages or coordination threads exist

### 4. Coordination does not require the human as message bus

Agents can coordinate directly:

```bash
./bin/pane board
./bin/pane ask <short-id> "question"
./bin/pane inbox
./bin/pane reply <message-id> "answer"
```

V1 ready requires targeting to be understandable enough that agents do not need long full session IDs every time.

### 5. Risky operations have useful guardrails

Agents can use:

```bash
./bin/pane git <args...>
```

and get relevant warnings before disruptive operations.

V1 ready does not require perfect semantic understanding, but warnings should be narrow enough that agents do not learn to ignore them.

### 6. The daemon feels dependable

A user or agent can run:

```bash
./bin/pane daemon status
./bin/pane daemon health
```

and understand whether Pane is running, which DB/socket it uses, and where lifecycle metadata lives.

V1 ready requires daemon startup/restart behavior to be unsurprising enough for daily use.

### 7. Docs are sufficient for another machine or agent

A fresh checkout should have enough committed docs for another computer or agent to continue work:

- `README.md` explains the product and current status
- `USE_CASES.md` explains the problems Pane solves
- `AGENTS.md` gives exact operating instructions
- `PROGRESS.md` says what is done and what is next
- `ARCHITECTURE.md` explains design rules and ownership

## Current V1-ready checkpoint

The main blocker was session lifecycle cleanup / board freshness. A first pass now exists:

- `pane close` closes the current session
- `pane sessions prune` closes stale active/idle sessions in the workspace
- `pane board` hides closed and first-pass stale sessions by default
- `pane history` still preserves durable past sessions

Pane is now ready for focused V1 dogfooding. If board freshness feels trustworthy with real panes, V1 can be considered "good enough to just use" for daily local agent memory.

Do not start V2 until the required V1 dogfood checklist in `PROGRESS.md` passes on the real default daemon/DB with multiple panes.

## After V1 ready

After this bar is met, Pane can shift from foundation-building to deeper quality work:

- richer overlap detection
- better git preflight based on file activity
- better summaries and history
- stronger daemon lifecycle hardening
- richer agent memory conventions
- provider/tool-specific integrations
