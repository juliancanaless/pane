# Pane Analysis Engine

This is the V3 Rust analysis engine scaffold.

`pane-analyze` parses source files with tree-sitter and emits JSON symbol tables and first-pass dependency edges. The Go side calls it as a subprocess through `internal/analysis.Client`, keeping the Rust engine independently testable and avoiding CGo/FFI during the scaffold phase.

## Build and test

From the repo root:

```bash
make build
cargo test --manifest-path analysis/Cargo.toml
```

`make build` copies the analyzer binary to:

```text
bin/pane-analyze
```

## Usage

```bash
bin/pane-analyze symbols internal/session/manager.go
bin/pane-analyze deps internal/session/manager.go
./bin/pane analyze symbols internal/session/manager.go
./bin/pane analyze deps internal/session/manager.go
./bin/pane analyze index internal/session/manager.go
./bin/pane analyze dependents github.com/juliancanalez/pane/internal/store
```

Symbol output shape:

```json
{
  "file": "internal/session/manager.go",
  "language": "go",
  "symbols": [
    {
      "name": "NewManager",
      "kind": "function",
      "start_line": 34,
      "end_line": 36
    }
  ]
}
```

Dependency output shape:

```json
{
  "file": "internal/session/manager.go",
  "language": "go",
  "dependencies": [
    {
      "target": "github.com/juliancanalez/pane/internal/store",
      "target_symbol": "",
      "kind": "import",
      "confidence": 0.9,
      "line": 12
    }
  ]
}
```

`pane analyze index <path...>` persists symbols and dependency edges into SQLite. `pane analyze dependents <target>` queries the persisted graph for files that depend on a module/path/symbol target.

## Current language support

- Go (`.go`)
- Python (`.py`)
- Rust (`.rs`)
- TypeScript (`.ts`, `.tsx`)

Dependency extraction is intentionally first-pass: imports/use/require edges are captured before deeper cross-language reference resolution and semantic warnings in later V3 work.
