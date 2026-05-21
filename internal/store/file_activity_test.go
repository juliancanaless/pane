package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/juliancanalez/pane/internal/activity"
	"github.com/juliancanalez/pane/internal/session"
)

func TestFileActivityStoreRecentBySession(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	if err := NewSessionStore(db).Save(ctx, session.Session{ID: "session-a", PaneID: "pane-a", WorkspaceRoot: "/workspace", StartedAt: 1, LastSeenAt: 1, Status: session.StatusActive}); err != nil {
		t.Fatalf("Save session returned error: %v", err)
	}
	store := NewFileActivityStore(db)
	for _, item := range []activity.FileActivity{
		{SessionID: "session-a", Path: "/workspace/a.go", EventType: activity.EventModified, Attribution: activity.AttributionHigh, Timestamp: 10},
		{SessionID: "session-a", Path: "/workspace/b.go", EventType: activity.EventModified, Attribution: activity.AttributionHigh, Timestamp: 20},
	} {
		if err := store.Save(ctx, item); err != nil {
			t.Fatalf("Save activity returned error: %v", err)
		}
	}
	recent, err := store.RecentBySession(ctx, "session-a", 0, 1)
	if err != nil {
		t.Fatalf("RecentBySession returned error: %v", err)
	}
	if len(recent) != 1 || recent[0].Path != "/workspace/b.go" {
		t.Fatalf("unexpected recent activity: %#v", recent)
	}
}
