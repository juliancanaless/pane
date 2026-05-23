# Pane v0.1.0

First shareable release of Pane: local shared memory and coordination for concurrent coding agents.

## Highlights

- Daemon-backed sessions, board, summary, history, and messaging
- File activity tracking, overlap warnings, and git preflight guardrails
- Rust/tree-sitter analyzer helper for symbols and dependency edges
- Semantic overlap warnings based on first-pass dependency data
- Activity decay and work-log history output
- Git worktree-aware repo scope with `pane board --repo` and `pane history --repo`
- Worker/child sessions with `pane spawn`
- Lineage view with `pane history --lineage`
- Agent state namespaces, global state, and `summary.*` startup context
- `pane setup` and `pane doctor`
- macOS/Linux CI with daemon smoke tests

## Install

```bash
curl -L -o pane.rb https://raw.githubusercontent.com/juliancanaless/pane/main/packaging/homebrew/pane.rb
brew install ./pane.rb
pane setup
pane doctor
```

Or from source:

```bash
git clone git@github.com:juliancanaless/pane.git
cd pane
make build
./bin/pane setup
./bin/pane doctor
```

## Known limitations

- Semantic overlap is first-pass import/package-level, not full symbol-reference/signature-aware analysis.
- Homebrew support is currently a formula file, not a dedicated tap.
- Long-running multi-day dogfood is still needed before calling Pane fully production-grade.
