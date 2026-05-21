package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/juliancanalez/pane/internal/session"
)

func TestGitEventStoreRecentByBranch(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := NewSessionStore(db).Save(ctx, session.Session{ID: "session-a", PaneID: "pane-a", WorkspaceRoot: "/workspace", StartedAt: 1, LastSeenAt: 1, Status: session.StatusActive}); err != nil {
		t.Fatalf("Save session returned error: %v", err)
	}
	store := NewGitEventStore(db)
	if err := store.Save(ctx, GitEvent{SessionID: "session-a", Command: "status", Subcommand: "status", Branch: "main", Timestamp: 10, Result: "ok"}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	events, err := store.RecentByBranch(ctx, "main", 0, 10)
	if err != nil {
		t.Fatalf("RecentByBranch returned error: %v", err)
	}
	if len(events) != 1 || events[0].Command != "status" {
		t.Fatalf("unexpected events: %#v", events)
	}
}
