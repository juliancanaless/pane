package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run([]string{"help"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("expected usage output, got %q", stdout.String())
	}
}

func TestStateNamespaceLines(t *testing.T) {
	items := []map[string]any{
		{"key": "alpha.memory", "session_id": "session-a", "updated_at": float64(10)},
		{"key": "alpha.notes", "session_id": "session-b", "updated_at": float64(20)},
		{"key": "beta", "session_id": "session-c", "updated_at": float64(5)},
	}

	got := namespaceLines(items)
	if len(got) != 2 {
		t.Fatalf("namespace count = %d", len(got))
	}
	if got[0].Namespace != "alpha" || got[0].Count != 2 || got[0].SessionID != "session-b" {
		t.Fatalf("alpha namespace = %#v", got[0])
	}
	if got[1].Namespace != "beta" || got[1].Count != 1 || got[1].SessionID != "session-c" {
		t.Fatalf("beta namespace = %#v", got[1])
	}
}

func TestParseStateScope(t *testing.T) {
	got := parseStateScope([]string{"--global", "agent.memory"})
	if got.Scope != "global" || len(got.Rest) != 1 || got.Rest[0] != "agent.memory" {
		t.Fatalf("parsed = %#v", got)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run([]string{"nope"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error")
	}
}
