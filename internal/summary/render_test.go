package summary

import (
	"strings"
	"testing"
	"time"

	"github.com/juliancanalez/pane/internal/messages"
	"github.com/juliancanalez/pane/internal/session"
)

func TestRenderSummaryWithPeer(t *testing.T) {
	value := StartupSummary{
		WorkspaceRoot: "/workspace",
		Current:       SessionLine{SessionID: "session-a", Status: session.StatusActive, Branch: "main", CWD: "/workspace/src", LastIntent: "refactoring auth", LastSeenAt: 100},
		Peers:         []SessionLine{{SessionID: "session-b", Status: session.StatusActive, Branch: "main", CWD: "/workspace/tests", LastIntent: "writing auth tests", LastSeenAt: 70}},
		Coordination: Coordination{
			UnreadMessages:  []messages.Message{{ID: "msg-1", FromSession: "session-b", Body: "Are you done?", CreatedAt: 100}},
			AwaitingReplies: 1,
		},
	}

	got := Render(value, time.Unix(130, 0))
	for _, want := range []string{
		"[Pane] Session summary",
		"Current session: session-a — active — main",
		"Intent: refactoring auth",
		"CWD: src",
		"Coordination:",
		"Unread messages: 1",
		"msg-1 from session-b (30s ago): Are you done?",
		"Awaiting replies: 1",
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

func TestRenderSummaryWithOverlap(t *testing.T) {
	value := StartupSummary{
		WorkspaceRoot: "/workspace",
		Current:       SessionLine{SessionID: "session-a", Status: session.StatusActive, Branch: "main", CWD: "/workspace/src", LastIntent: "refactoring auth", LastSeenAt: 100},
		Peers:         []SessionLine{{SessionID: "session-b", Status: session.StatusActive, Branch: "main", CWD: "/workspace/tests", LastIntent: "writing auth tests", LastSeenAt: 70}},
		Overlaps: []OverlapInfo{
			{PeerSessionID: "session-b", SharedFiles: []string{"/workspace/src/auth.go", "/workspace/src/auth_test.go"}},
		},
	}

	got := Render(value, time.Unix(130, 0))
	for _, want := range []string{
		"Overlap:",
		"session-b shares:",
		"src/auth.go",
		"src/auth_test.go",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("Render output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderSummaryWithActivitySummary(t *testing.T) {
	value := StartupSummary{
		WorkspaceRoot:     "/workspace",
		Current:           SessionLine{SessionID: "session-a", Status: session.StatusActive, CWD: "/workspace"},
		ActivitySummaries: []string{"3 files compressed (2h–72h): internal/daemon"},
	}

	got := Render(value, time.Unix(130, 0))
	if !strings.Contains(got, "Activity summary: 3 files compressed (2h–72h): internal/daemon") {
		t.Fatalf("Render output missing activity summary:\n%s", got)
	}
}

func TestRenderSummaryWithStateItems(t *testing.T) {
	value := StartupSummary{
		WorkspaceRoot: "/workspace",
		Current:       SessionLine{SessionID: "session-a", Status: session.StatusActive},
		StateItems:    []StateItem{{Key: "summary.note", ValueJSON: `{"text":"remember API rename"}`, SessionID: "session-a", UpdatedAt: 100}},
	}

	got := Render(value, time.Unix(130, 0))
	for _, want := range []string{"Shared state:", "summary.note", "remember API rename", "session-a"} {
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
