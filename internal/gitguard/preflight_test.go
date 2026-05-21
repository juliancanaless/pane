package gitguard

import (
	"strings"
	"testing"

	"github.com/juliancanalez/pane/internal/session"
)

func TestPreflightWarnsForSameBranchPeer(t *testing.T) {
	result := Preflight(PreflightInput{
		Intent:         Parse([]string{"rebase", "main"}),
		CurrentSession: session.Session{ID: "session-a", Branch: "feature"},
		ActiveSessions: []session.Session{{ID: "session-a", Branch: "feature"}, {ID: "session-b", Branch: "feature"}},
	})
	if len(result.Warnings) == 0 {
		t.Fatal("expected warning")
	}
}

func TestPreflightWarnsForFileOverlap(t *testing.T) {
	result := Preflight(PreflightInput{
		Intent:         Parse([]string{"commit", "-m", "wip"}),
		CurrentSession: session.Session{ID: "session-a", Branch: "feature"},
		ActiveSessions: []session.Session{{ID: "session-a", Branch: "feature"}, {ID: "session-b", Branch: "develop"}},
		FileOverlaps: []FileOverlap{
			{PeerSessionID: "session-b", SharedFiles: []string{"src/auth.go", "src/session.go"}},
		},
	})
	if len(result.Warnings) == 0 {
		t.Fatal("expected file overlap warning")
	}
	found := false
	for _, w := range result.Warnings {
		if strings.Contains(w, "overlapping files") && strings.Contains(w, "session-b") && strings.Contains(w, "src/auth.go") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected file overlap warning mentioning session-b and shared files, got: %v", result.Warnings)
	}
}

func TestPreflightNoWarningWithoutOverlap(t *testing.T) {
	result := Preflight(PreflightInput{
		Intent:         Parse([]string{"commit", "-m", "wip"}),
		CurrentSession: session.Session{ID: "session-a", Branch: "feature"},
		ActiveSessions: []session.Session{{ID: "session-a", Branch: "feature"}, {ID: "session-b", Branch: "develop"}},
	})
	if len(result.Warnings) != 0 {
		t.Fatalf("expected no warnings, got: %v", result.Warnings)
	}
}

func TestPreflightBlocksForceWithSameBranchPeer(t *testing.T) {
	result := Preflight(PreflightInput{
		Intent:         Parse([]string{"push", "--force-with-lease"}),
		CurrentSession: session.Session{ID: "session-a", Branch: "feature"},
		ActiveSessions: []session.Session{{ID: "session-a", Branch: "feature"}, {ID: "session-b", Branch: "feature"}},
	})
	if !result.Block {
		t.Fatal("expected block")
	}
}
