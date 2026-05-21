package gitguard

import (
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
