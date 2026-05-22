package store

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type AnalysisSymbol struct {
	Name      string
	Kind      string
	StartLine int
	EndLine   int
}

type DependencyEdge struct {
	SourceFile   string
	Target       string
	TargetSymbol string
	Kind         string
	Confidence   float64
}

type FileAnalysis struct {
	WorkspaceRoot string
	File          string
	Language      string
	Symbols       []AnalysisSymbol
	Dependencies  []DependencyEdge
}

type Dependent struct {
	SourceFile   string
	Target       string
	TargetSymbol string
	Kind         string
	Confidence   float64
}

type AnalysisStore struct {
	db *sql.DB
}

func NewAnalysisStore(db *sql.DB) AnalysisStore {
	return AnalysisStore{db: db}
}

func (s AnalysisStore) UpsertFile(ctx context.Context, analysis FileAnalysis) error {
	now := time.Now().UnixMilli()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM analysis_symbols WHERE workspace_root = ? AND file = ?`, analysis.WorkspaceRoot, analysis.File); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM dependency_edges WHERE workspace_root = ? AND source_file = ?`, analysis.WorkspaceRoot, analysis.File); err != nil {
		return err
	}
	for _, symbol := range analysis.Symbols {
		if _, err := tx.ExecContext(ctx, `INSERT INTO analysis_symbols (workspace_root, file, language, name, kind, start_line, end_line, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, analysis.WorkspaceRoot, analysis.File, analysis.Language, symbol.Name, symbol.Kind, symbol.StartLine, symbol.EndLine, now); err != nil {
			return err
		}
	}
	for _, edge := range analysis.Dependencies {
		if _, err := tx.ExecContext(ctx, `INSERT INTO dependency_edges (workspace_root, source_file, target, target_symbol, kind, confidence, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, analysis.WorkspaceRoot, analysis.File, edge.Target, edge.TargetSymbol, edge.Kind, edge.Confidence, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s AnalysisStore) SymbolsByFile(ctx context.Context, workspaceRoot string, files []string) (map[string][]AnalysisSymbol, error) {
	if len(files) == 0 {
		return nil, nil
	}
	query := `SELECT file, name, kind, start_line, end_line FROM analysis_symbols WHERE workspace_root = ? AND file IN (` + placeholders(len(files)) + `) ORDER BY file, start_line`
	args := append([]any{workspaceRoot}, stringsToAny(files)...)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]AnalysisSymbol)
	for rows.Next() {
		var file string
		var symbol AnalysisSymbol
		if err := rows.Scan(&file, &symbol.Name, &symbol.Kind, &symbol.StartLine, &symbol.EndLine); err != nil {
			return nil, err
		}
		result[file] = append(result[file], symbol)
	}
	return result, rows.Err()
}

func (s AnalysisStore) EdgesBySourceFiles(ctx context.Context, workspaceRoot string, files []string) ([]DependencyEdge, error) {
	if len(files) == 0 {
		return nil, nil
	}
	query := `SELECT source_file, target, target_symbol, kind, confidence FROM dependency_edges WHERE workspace_root = ? AND source_file IN (` + placeholders(len(files)) + `) ORDER BY source_file, confidence DESC`
	args := append([]any{workspaceRoot}, stringsToAny(files)...)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var edges []DependencyEdge
	for rows.Next() {
		var edge DependencyEdge
		if err := rows.Scan(&edge.SourceFile, &edge.Target, &edge.TargetSymbol, &edge.Kind, &edge.Confidence); err != nil {
			return nil, err
		}
		edges = append(edges, edge)
	}
	return edges, rows.Err()
}

func (s AnalysisStore) Dependents(ctx context.Context, workspaceRoot, target, targetSymbol string) ([]Dependent, error) {
	query := `SELECT source_file, target, target_symbol, kind, confidence FROM dependency_edges WHERE workspace_root = ? AND (target = ? OR target_symbol = ? OR target LIKE ?) ORDER BY confidence DESC, source_file`
	rows, err := s.db.QueryContext(ctx, query, workspaceRoot, target, targetSymbol, "%"+target+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dependents []Dependent
	for rows.Next() {
		var dep Dependent
		if err := rows.Scan(&dep.SourceFile, &dep.Target, &dep.TargetSymbol, &dep.Kind, &dep.Confidence); err != nil {
			return nil, err
		}
		dependents = append(dependents, dep)
	}
	return dependents, rows.Err()
}

func placeholders(count int) string {
	values := make([]string, count)
	for i := range values {
		values[i] = "?"
	}
	return strings.Join(values, ",")
}

func stringsToAny(values []string) []any {
	args := make([]any, 0, len(values))
	for _, value := range values {
		args = append(args, value)
	}
	return args
}
