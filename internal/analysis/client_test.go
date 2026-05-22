package analysis

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestClientSymbolsUsesAnalyzerJSON(t *testing.T) {
	dir := t.TempDir()
	analyzer := filepath.Join(dir, "pane-analyze")
	if err := os.WriteFile(analyzer, []byte("#!/bin/sh\nprintf '%s\n' '{\"file\":\"sample.go\",\"language\":\"go\",\"symbols\":[{\"name\":\"Hello\",\"kind\":\"function\",\"start_line\":1,\"end_line\":1}]}'\n"), 0o755); err != nil {
		t.Fatalf("write analyzer: %v", err)
	}

	table, err := (Client{AnalyzerPath: analyzer}).Symbols(context.Background(), "sample.go")
	if err != nil {
		t.Fatalf("Symbols returned error: %v", err)
	}
	if table.Language != "go" || len(table.Symbols) != 1 || table.Symbols[0].Name != "Hello" {
		t.Fatalf("unexpected table: %#v", table)
	}
}

func TestClientDependenciesUsesAnalyzerJSON(t *testing.T) {
	dir := t.TempDir()
	analyzer := filepath.Join(dir, "pane-analyze")
	if err := os.WriteFile(analyzer, []byte("#!/bin/sh\nprintf '%s\n' '{\"file\":\"sample.go\",\"language\":\"go\",\"dependencies\":[{\"target\":\"fmt\",\"target_symbol\":\"\",\"kind\":\"import\",\"confidence\":0.9,\"line\":3}]}'\n"), 0o755); err != nil {
		t.Fatalf("write analyzer: %v", err)
	}

	graph, err := (Client{AnalyzerPath: analyzer}).Dependencies(context.Background(), "sample.go")
	if err != nil {
		t.Fatalf("Dependencies returned error: %v", err)
	}
	if graph.Language != "go" || len(graph.Dependencies) != 1 || graph.Dependencies[0].Target != "fmt" {
		t.Fatalf("unexpected graph: %#v", graph)
	}
}
