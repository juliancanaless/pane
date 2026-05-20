package messages

import (
	"strings"
	"testing"
	"time"
)

func TestRenderInbox(t *testing.T) {
	got := RenderInbox([]Message{{ID: "msg-1", ThreadID: "msg-1", FromSession: "session-a", Body: "Are you done?", CreatedAt: 100}}, time.Unix(130, 0))
	for _, want := range []string{"[Pane] Inbox", "Messages: 1", "msg-1 — from session-a — 30s ago", "Are you done?"} {
		if !strings.Contains(got, want) {
			t.Fatalf("RenderInbox missing %q:\n%s", want, got)
		}
	}
}

func TestRenderEmptyInbox(t *testing.T) {
	got := RenderInbox(nil, time.Unix(130, 0))
	if !strings.Contains(got, "No unread coordination messages") {
		t.Fatalf("RenderInbox missing empty state:\n%s", got)
	}
}
