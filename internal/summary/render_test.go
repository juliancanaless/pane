package summary

import (
	"strings"
	"testing"
	"time"

	"github.com/juliancanalez/pane/internal/session"
)

func TestRenderSummaryWithPeer(t *testing.T) {
	value := StartupSummary{
		WorkspaceRoot: "/workspace",
		Current:       SessionLine{SessionID: "session-a", Status: session.StatusActive, Branch: "main", CWD: "/workspace/src", LastIntent: "refactoring auth", LastSeenAt: 100},
		Peers:         []SessionLine{{SessionID: "session-b", Status: session.StatusActive, Branch: "main", CWD: "/workspace/tests", LastIntent: "writing auth tests", LastSeenAt: 70}},
	}

	got := Render(value, time.Unix(130, 0))
	for _, want := range []string{
		"[Pane] Session summary",
		"Current session: session-a — active — main",
		"Intent: refactoring auth",
		"CWD: src",
		"Other sessions: 1",
		"session-b — active — main",
		"Intent: writing auth tests",
		"Last seen: 1m ago",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Render output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderSummaryWithoutPeers(t *testing.T) {
	value := StartupSummary{WorkspaceRoot: "/workspace", Current: SessionLine{SessionID: "session-a", Status: session.StatusActive}}
	got := Render(value, time.Unix(130, 0))
	if !strings.Contains(got, "None currently visible") {
		t.Fatalf("Render output missing empty peer state:\n%s", got)
	}
}
