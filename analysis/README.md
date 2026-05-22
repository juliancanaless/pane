# Pane Analysis Engine

This is the V3 Rust analysis scaffold.

`pane-analyze` parses source files with tree-sitter and emits a JSON symbol table. The Go side calls it as a subprocess through `internal/analysis.Client`, keeping the Rust engine independently testable and avoiding CGo/FFI during the scaffold phase.

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
./bin/pane analyze symbols internal/session/manager.go
```

Output shape:

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

## Current language support

- Go (`.go`)
- Python (`.py`)
- Rust (`.rs`)
- TypeScript (`.ts`, `.tsx`)

This is intentionally a scaffold. V3.2 should build dependency graphs and persist analysis results in Pane's SQLite store.
