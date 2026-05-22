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
