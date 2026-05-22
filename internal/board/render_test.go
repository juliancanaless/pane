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
			{ID: "session-a", Status: session.StatusActive, Branch: "main", CWD: "/workspace/src", LastIntent: "working on auth", LastSeenAt: 100, UnreadMessages: 2, AwaitingReplies: 1},
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
		"Coordination: 2 unread, 1 awaiting reply",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Render output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderBoardWithOverlap(t *testing.T) {
	value := Board{
		WorkspaceRoot: "/workspace",
		Sessions: []Session{
			{ID: "session-aaa", ShortID: "aaa", Status: session.StatusActive, Branch: "main", CWD: "/workspace", LastIntent: "working on auth", LastSeenAt: 100},
			{ID: "session-bbb", ShortID: "bbb", Status: session.StatusActive, Branch: "main", CWD: "/workspace", LastIntent: "writing tests", LastSeenAt: 100},
		},
		Overlaps: []OverlapInfo{
			{SessionA: "session-aaa", SessionB: "session-bbb", SharedFiles: []string{"/workspace/src/auth.go"}},
		},
	}

	got := Render(value, time.Unix(130, 0))
	for _, want := range []string{
		"Overlap:",
		"aaa ↔ bbb",
		"src/auth.go",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Render output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderBoardWithActivitySummary(t *testing.T) {
	value := Board{
		WorkspaceRoot: "/workspace",
		Sessions: []Session{
			{ID: "session-aaa", ShortID: "aaa", Status: session.StatusActive, Branch: "main", CWD: "/workspace", LastIntent: "long refactor", LastSeenAt: 100,
				ActivitySummaries: []string{"4 files in summary tier (5m–2h): internal/store"}},
		},
	}

	got := Render(value, time.Unix(130, 0))
	if !strings.Contains(got, "Activity summary: 4 files in summary tier (5m–2h): internal/store") {
		t.Fatalf("Render output missing activity summary:\n%s", got)
	}
}

func TestRenderBoardWithHotDirsAndGitEvents(t *testing.T) {
	value := Board{
		WorkspaceRoot: "/workspace",
		Sessions: []Session{
			{ID: "session-aaa", ShortID: "aaa", Status: session.StatusActive, Branch: "main", CWD: "/workspace", LastIntent: "auth work", LastSeenAt: 100,
				HotDirectories: []string{"/workspace/src/auth"}},
		},
		RecentGitEvents: []GitEventInfo{
			{SessionShortID: "aaa", Command: "commit -m 'wip'", Timestamp: 95},
		},
	}

	got := Render(value, time.Unix(130, 0))
	for _, want := range []string{
		"Hot dirs: src/auth",
		"Recent git:",
		"aaa — commit -m 'wip'",
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
