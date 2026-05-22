package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestAnalysisStoreUpsertAndDependents(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	store := NewAnalysisStore(db)
	ctx := context.Background()
	analysis := FileAnalysis{
		WorkspaceRoot: "/workspace",
		File:          "auth/service.go",
		Language:      "go",
		Symbols: []AnalysisSymbol{
			{Name: "ValidateToken", Kind: "function", StartLine: 10, EndLine: 20},
		},
		Dependencies: []DependencyEdge{
			{SourceFile: "auth/service.go", Target: "crypto", TargetSymbol: "Sign", Kind: "import", Confidence: 0.9},
		},
	}
	if err := store.UpsertFile(ctx, analysis); err != nil {
		t.Fatalf("upsert file: %v", err)
	}

	dependents, err := store.Dependents(ctx, "/workspace", "crypto", "crypto")
	if err != nil {
		t.Fatalf("dependents: %v", err)
	}
	if len(dependents) != 1 || dependents[0].SourceFile != "auth/service.go" {
		t.Fatalf("unexpected dependents: %#v", dependents)
	}

	analysis.Dependencies = nil
	if err := store.UpsertFile(ctx, analysis); err != nil {
		t.Fatalf("replace file analysis: %v", err)
	}
	dependents, err = store.Dependents(ctx, "/workspace", "crypto", "crypto")
	if err != nil {
		t.Fatalf("dependents after replace: %v", err)
	}
	if len(dependents) != 0 {
		t.Fatalf("expected old edges to be replaced, got %#v", dependents)
	}
}

func TestAnalysisStoreSymbolsAndEdgesByFile(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	analysisStore := NewAnalysisStore(db)
	ctx := context.Background()
	if err := analysisStore.UpsertFile(ctx, FileAnalysis{
		WorkspaceRoot: "/workspace",
		File:          "auth/handler.go",
		Language:      "go",
		Symbols:       []AnalysisSymbol{{Name: "Handle", Kind: "function", StartLine: 4, EndLine: 8}},
		Dependencies:  []DependencyEdge{{Target: "github.com/example/project/crypto", TargetSymbol: "", Kind: "import", Confidence: 0.9}},
	}); err != nil {
		t.Fatalf("upsert handler: %v", err)
	}
	if err := analysisStore.UpsertFile(ctx, FileAnalysis{
		WorkspaceRoot: "/workspace",
		File:          "crypto/token.go",
		Language:      "go",
		Symbols:       []AnalysisSymbol{{Name: "ValidateToken", Kind: "function", StartLine: 10, EndLine: 20}},
	}); err != nil {
		t.Fatalf("upsert crypto: %v", err)
	}

	symbols, err := analysisStore.SymbolsByFile(ctx, "/workspace", []string{"crypto/token.go"})
	if err != nil {
		t.Fatalf("symbols by file: %v", err)
	}
	if len(symbols["crypto/token.go"]) != 1 || symbols["crypto/token.go"][0].Name != "ValidateToken" {
		t.Fatalf("unexpected symbols: %#v", symbols)
	}

	edges, err := analysisStore.EdgesBySourceFiles(ctx, "/workspace", []string{"auth/handler.go"})
	if err != nil {
		t.Fatalf("edges by source file: %v", err)
	}
	if len(edges) != 1 || edges[0].SourceFile != "auth/handler.go" || edges[0].Target != "github.com/example/project/crypto" {
		t.Fatalf("unexpected edges: %#v", edges)
	}
}
