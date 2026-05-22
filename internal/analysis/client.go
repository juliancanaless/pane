package analysis

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type SymbolTable struct {
	File     string   `json:"file"`
	Language string   `json:"language"`
	Symbols  []Symbol `json:"symbols"`
}

type DependencyGraph struct {
	File         string       `json:"file"`
	Language     string       `json:"language"`
	Dependencies []Dependency `json:"dependencies"`
}

type Dependency struct {
	Target       string  `json:"target"`
	TargetSymbol string  `json:"target_symbol"`
	Kind         string  `json:"kind"`
	Confidence   float64 `json:"confidence"`
	Line         int     `json:"line"`
}

type Symbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

type Client struct {
	AnalyzerPath string
}

func (c Client) Symbols(ctx context.Context, file string) (SymbolTable, error) {
	var table SymbolTable
	if err := c.run(ctx, &table, "symbols", file); err != nil {
		return SymbolTable{}, err
	}
	return table, nil
}

func (c Client) Dependencies(ctx context.Context, file string) (DependencyGraph, error) {
	var graph DependencyGraph
	if err := c.run(ctx, &graph, "deps", file); err != nil {
		return DependencyGraph{}, err
	}
	return graph, nil
}

func (c Client) run(ctx context.Context, into any, args ...string) error {
	analyzer := c.AnalyzerPath
	if analyzer == "" {
		analyzer = defaultAnalyzerPath()
	}
	cmd := exec.CommandContext(ctx, analyzer, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("analysis failed: %s", string(exitErr.Stderr))
		}
		return err
	}
	return json.Unmarshal(output, into)
}

func defaultAnalyzerPath() string {
	if value := os.Getenv("PANE_ANALYZER_PATH"); value != "" {
		return value
	}
	return filepath.Join("bin", "pane-analyze")
}
