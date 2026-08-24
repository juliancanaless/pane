package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/juliancanalez/pane/internal/daemon"
)

// recordForegroundRuns substitutes the blocking-daemon seam so flag parsing can
// be tested without running a daemon.
func recordForegroundRuns(t *testing.T) *int {
	t.Helper()
	count := 0
	original := runDaemonForeground
	runDaemonForeground = func(daemon.Config) error {
		count++
		return nil
	}
	t.Cleanup(func() { runDaemonForeground = original })
	return &count
}

func TestRunDaemonStartBackgroundsByDefault(t *testing.T) {
	tempDaemonPaths(t)
	started := recordDaemonStarts(t, nil)
	foreground := recordForegroundRuns(t)

	var stdout, stderr bytes.Buffer
	if err := Run([]string{"daemon", "start"}, &stdout, &stderr); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(*started) != 1 {
		t.Fatalf("background starts = %d, want 1", len(*started))
	}
	if *foreground != 0 {
		t.Fatalf("foreground runs = %d, want 0", *foreground)
	}
}

func TestRunDaemonStartForegroundRunsBlockingDaemon(t *testing.T) {
	tempDaemonPaths(t)
	started := recordDaemonStarts(t, nil)
	foreground := recordForegroundRuns(t)

	for _, flag := range []string{"--foreground", "-f"} {
		var stdout, stderr bytes.Buffer
		if err := Run([]string{"daemon", "start", flag}, &stdout, &stderr); err != nil {
			t.Fatalf("%s: Run returned error: %v", flag, err)
		}
	}
	if *foreground != 2 {
		t.Fatalf("foreground runs = %d, want 2", *foreground)
	}
	if len(*started) != 0 {
		t.Fatalf("background starts = %d, want 0", len(*started))
	}
}

func TestRunDaemonStartAcceptsBackgroundFlagAsNoOp(t *testing.T) {
	tempDaemonPaths(t)
	started := recordDaemonStarts(t, nil)
	foreground := recordForegroundRuns(t)

	for _, flag := range []string{"--background", "-b"} {
		var stdout, stderr bytes.Buffer
		if err := Run([]string{"daemon", "start", flag}, &stdout, &stderr); err != nil {
			t.Fatalf("%s: Run returned error: %v", flag, err)
		}
	}
	if len(*started) != 2 {
		t.Fatalf("background starts = %d, want 2", len(*started))
	}
	if *foreground != 0 {
		t.Fatalf("foreground runs = %d, want 0", *foreground)
	}
}

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

func TestRunDocsAgents(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run([]string{"docs", "agents"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	for _, want := range []string{"Pane agent operating contract", "pane intent", "human"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("docs missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run([]string{"nope"}, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected an error")
	}
}
