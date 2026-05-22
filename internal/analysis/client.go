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
	analyzer := c.AnalyzerPath
	if analyzer == "" {
		analyzer = defaultAnalyzerPath()
	}
	cmd := exec.CommandContext(ctx, analyzer, "symbols", file)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return SymbolTable{}, fmt.Errorf("analysis failed: %s", string(exitErr.Stderr))
		}
		return SymbolTable{}, err
	}
	var table SymbolTable
	if err := json.Unmarshal(output, &table); err != nil {
		return SymbolTable{}, err
	}
	return table, nil
}

func defaultAnalyzerPath() string {
	if value := os.Getenv("PANE_ANALYZER_PATH"); value != "" {
		return value
	}
	return filepath.Join("bin", "pane-analyze")
}
