package board

import (
	"strings"
	"testing"
	"time"

	"github.com/juliancanalez/pane/internal/session"
)

func TestRenderBoard(t *testing.T) {
	value := Board{
		WorkspaceRoot: "/workspace",
		Sessions: []Session{
			{ID: "session-a", Status: session.StatusActive, Branch: "main", CWD: "/workspace/src", LastIntent: "working on auth", LastSeenAt: 100},
		},
	}

	got := Render(value, time.Unix(130, 0))
	for _, want := range []string{
		"[Pane] Workspace board",
		"Workspace: /workspace",
		"Sessions: 1",
		"session-a — active — main",
		"Intent: working on auth",
		"CWD: src",
		"Last seen: 30s ago",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Render output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderEmptyBoard(t *testing.T) {
	got := Render(Board{WorkspaceRoot: "/workspace"}, time.Unix(130, 0))
	if !strings.Contains(got, "No active sessions found") {
		t.Fatalf("Render output missing empty state:\n%s", got)
	}
}
