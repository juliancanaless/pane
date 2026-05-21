package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/juliancanalez/pane/internal/activity"
	"github.com/juliancanalez/pane/internal/session"
)

func TestFileActivityStoreOverlapByWorkspace(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()
	ctx := context.Background()
	sessionStore := NewSessionStore(db)
	for _, s := range []session.Session{
		{ID: "session-a", PaneID: "pane-a", WorkspaceRoot: "/workspace", StartedAt: 1, LastSeenAt: 1, Status: session.StatusActive},
		{ID: "session-b", PaneID: "pane-b", WorkspaceRoot: "/workspace", StartedAt: 1, LastSeenAt: 1, Status: session.StatusActive},
	} {
		if err := sessionStore.Save(ctx, s); err != nil {
			t.Fatalf("Save session returned error: %v", err)
		}
	}
	store := NewFileActivityStore(db)
	for _, item := range []activity.FileActivity{
		{SessionID: "session-a", Path: "/workspace/shared.go", EventType: activity.EventModified, Attribution: activity.AttributionHigh, Timestamp: 10},
		{SessionID: "session-b", Path: "/workspace/shared.go", EventType: activity.EventModified, Attribution: activity.AttributionHigh, Timestamp: 11},
		{SessionID: "session-a", Path: "/workspace/solo.go", EventType: activity.EventModified, Attribution: activity.AttributionHigh, Timestamp: 12},
	} {
		if err := store.Save(ctx, item); err != nil {
			t.Fatalf("Save activity returned error: %v", err)
		}
	}
	result, err := store.OverlapByWorkspace(ctx, "/workspace", 0)
	if err != nil {
		t.Fatalf("OverlapByWorkspace returned error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 overlapping path, got %d: %v", len(result), result)
	}
	ids, ok := result["/workspace/shared.go"]
	if !ok {
		t.Fatalf("expected /workspace/shared.go in result, got: %v", result)
	}
	if len(ids) != 2 || ids[0] != "session-a" || ids[1] != "session-b" {
		t.Fatalf("expected [session-a session-b], got %v", ids)
	}
}

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
