# Pane scaffolding plan

## Decisions

- Language: Go
- CLI parsing: Go standard library/manual routing for V1
- SQLite driver: pure-Go SQLite (`modernc.org/sqlite`)
- Tests: included from the start

## Current scaffold

The repository is structured around the V1 boundaries from the architecture doc:

- `cmd/pane`: binary entry point
- `internal/cli`: command routing and user-facing command boundaries
- `internal/daemon`: daemon configuration and future socket server
- `internal/protocol`: request/response protocol types
- `internal/session`: pane/TTY session identity and lifecycle types
- `internal/board`: shared awareness board model and renderer
- `internal/summary`: session-start summary models
- `internal/gitguard`: git command parsing and preflight classification
- `internal/activity`: file activity model
- `internal/messages`: async message model
- `internal/store`: SQLite open/migration foundation
- `internal/summary`: startup summary model

## First implementation slice

Implemented in the current scaffold:

1. `pane init` resolves:
   - workspace root via `git rev-parse --show-toplevel`
   - current branch via `git branch --show-current`
   - pane ID via Zellij, tmux, or TTY fallback
2. Session init/resume/status/intent logic is represented in `internal/session`.
3. Sessions are persisted in SQLite through `internal/store`.
4. `pane status` returns the current pane/workspace session after `pane init`.
5. `pane intent <text>` updates the session's stated intent.

Still required before validation:

1. Install Go locally.
2. Run `go mod tidy` to generate `go.sum`.
3. Run `make test` and `make build`.

## Daemon socket slice

Implemented after the first session slice:

1. Length-prefixed JSON protocol codec.
2. Unix socket daemon server.
3. Daemon client for request/response calls.
4. `pane daemon start` foreground server mode.
5. `pane daemon health` over the socket.
6. `pane daemon stop` over the socket.

## Shared board slice

Implemented after the daemon socket slice:

1. `internal/board` model and text renderer.
2. Store support for listing active/idle sessions by workspace.
3. `pane board` command showing active sessions, status, branch, intent, cwd, and last seen.
4. Tests for board rendering and session listing.

## Daemon-owned session/board slice

Implemented after the shared board slice:

1. Daemon handlers for session init, status, intent, and board requests.
2. CLI session/board commands now call the daemon instead of opening SQLite directly.
3. Handler tests for daemon-backed session and board flows.
4. Local smoke test for daemon start + init + intent + status + board + stop.

## Summary slice

Implemented after daemon-owned session/board flows:

1. `internal/summary` model and renderer.
2. Daemon `GetSummary` handler.
3. `pane summary` command.
4. Tests for summary rendering and daemon summary handler.

## Next useful slice

1. Implement `pane ask`, `pane inbox`, and `pane reply` against SQLite through the daemon.
2. Surface unread/open messages in `pane board` and `pane summary`.
3. Implement `pane git` passthrough to the real git binary.
4. Add daemon-backed git preflight warnings.

## Validation commands

```bash
make test
make build
./bin/pane help
./bin/pane git status
```

Go is installed locally now. `go test ./...` and `make build` have passed.
