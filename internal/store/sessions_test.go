package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/juliancanalez/pane/internal/session"
)

func TestSessionStorePersistsParentSession(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	store := NewSessionStore(db)
	ctx := context.Background()
	if err := store.Save(ctx, session.Session{ID: "session-parent", PaneID: "pane-1", WorkspaceRoot: "/workspace", CWD: "/workspace", Branch: "main", StartedAt: 1, LastSeenAt: 1, Status: session.StatusActive}); err != nil {
		t.Fatalf("Save parent returned error: %v", err)
	}
	if err := store.Save(ctx, session.Session{ID: "session-child", PaneID: "pane-2", WorkspaceRoot: "/workspace", CWD: "/workspace", Branch: "main", StartedAt: 2, LastSeenAt: 2, Status: session.StatusActive, ParentID: "session-parent"}); err != nil {
		t.Fatalf("Save child returned error: %v", err)
	}

	got, err := store.FindByID(ctx, "session-child")
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if got.ParentID != "session-parent" {
		t.Fatalf("ParentID = %q", got.ParentID)
	}
}

func TestListActiveByWorkspace(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	store := NewSessionStore(db)
	ctx := context.Background()
	values := []session.Session{
		{ID: "session-active", PaneID: "pane-1", WorkspaceRoot: "/workspace", CWD: "/workspace", Branch: "main", LastIntent: "active work", StartedAt: 1, LastSeenAt: 3, Status: session.StatusActive},
		{ID: "session-idle", PaneID: "pane-2", WorkspaceRoot: "/workspace", CWD: "/workspace/tests", Branch: "main", LastIntent: "idle work", StartedAt: 1, LastSeenAt: 2, Status: session.StatusIdle},
		{ID: "session-closed", PaneID: "pane-3", WorkspaceRoot: "/workspace", CWD: "/workspace", Branch: "main", StartedAt: 1, LastSeenAt: 4, Status: session.StatusClosed},
		{ID: "session-other", PaneID: "pane-4", WorkspaceRoot: "/other", CWD: "/other", Branch: "main", StartedAt: 1, LastSeenAt: 5, Status: session.StatusActive},
	}
	for _, value := range values {
		if err := store.Save(ctx, value); err != nil {
			t.Fatalf("Save returned error: %v", err)
		}
	}

	got, err := store.ListActiveByWorkspace(ctx, "/workspace")
	if err != nil {
		t.Fatalf("ListActiveByWorkspace returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d sessions, want 2: %#v", len(got), got)
	}
	if got[0].ID != "session-active" || got[1].ID != "session-idle" {
		t.Fatalf("unexpected order/results: %#v", got)
	}
}
