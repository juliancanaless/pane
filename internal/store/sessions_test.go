package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/juliancanalez/pane/internal/session"
)

func TestSessionStorePersistsRepoIdentity(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	store := NewSessionStore(db)
	ctx := context.Background()
	value := session.Session{ID: "session-a", PaneID: "pane-a", WorkspaceRoot: "/workspace-a", CWD: "/workspace-a", Branch: "main", RepoID: "/repo/.git", GitCommonDir: "/repo/.git", StartedAt: 1, LastSeenAt: 1, Status: session.StatusActive}
	if err := store.Save(ctx, value); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := store.FindByID(ctx, "session-a")
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if got.RepoID != "/repo/.git" || got.GitCommonDir != "/repo/.git" {
		t.Fatalf("repo identity = %q common = %q", got.RepoID, got.GitCommonDir)
	}
}

func TestSessionStoreFindsByAgentSession(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	store := NewSessionStore(db)
	ctx := context.Background()
	value := session.Session{ID: "session-a", PaneID: "pane-a", WorkspaceRoot: "/workspace-a", StartedAt: 1, LastSeenAt: 1, Status: session.StatusActive, AgentSessionID: "claude-abc"}
	if err := store.Save(ctx, value); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	got, err := store.FindByAgentSession(ctx, "claude-abc")
	if err != nil {
		t.Fatalf("FindByAgentSession returned error: %v", err)
	}
	if got.ID != "session-a" || got.AgentSessionID != "claude-abc" {
		t.Fatalf("got session %q agent %q", got.ID, got.AgentSessionID)
	}

	// A save without an agent id must not clear the binding.
	value.AgentSessionID = ""
	value.LastSeenAt = 2
	if err := store.Save(ctx, value); err != nil {
		t.Fatalf("second Save returned error: %v", err)
	}
	got, err = store.FindByAgentSession(ctx, "claude-abc")
	if err != nil {
		t.Fatalf("FindByAgentSession after re-save returned error: %v", err)
	}
	if got.ID != "session-a" {
		t.Fatalf("binding lost after agent-less save; got %q", got.ID)
	}

	if _, err := store.FindByAgentSession(ctx, "missing"); err != session.ErrNotFound {
		t.Fatalf("expected ErrNotFound for unknown agent session, got %v", err)
	}
}

func TestSessionStoreListsActiveAllAcrossWorkspaces(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	store := NewSessionStore(db)
	ctx := context.Background()
	values := []session.Session{
		{ID: "session-a", PaneID: "pane-a", WorkspaceRoot: "/repo-one", RepoID: "/repo-one/.git", StartedAt: 1, LastSeenAt: 30, Status: session.StatusActive},
		{ID: "session-b", PaneID: "pane-b", WorkspaceRoot: "/repo-two", RepoID: "/repo-two/.git", StartedAt: 1, LastSeenAt: 20, Status: session.StatusIdle},
		{ID: "session-c", PaneID: "pane-c", WorkspaceRoot: "/repo-three", RepoID: "/repo-three/.git", StartedAt: 1, LastSeenAt: 10, Status: session.StatusClosed},
	}
	for _, value := range values {
		if err := store.Save(ctx, value); err != nil {
			t.Fatalf("Save returned error: %v", err)
		}
	}
	got, err := store.ListActiveAll(ctx)
	if err != nil {
		t.Fatalf("ListActiveAll returned error: %v", err)
	}
	// Spans workspaces, excludes closed, ordered by last_seen_at desc.
	if len(got) != 2 || got[0].ID != "session-a" || got[1].ID != "session-b" {
		t.Fatalf("unexpected sessions: %#v", got)
	}
}

func TestSessionStoreListsActiveByRepo(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	store := NewSessionStore(db)
	ctx := context.Background()
	values := []session.Session{
		{ID: "session-a", PaneID: "pane-a", WorkspaceRoot: "/worktree-a", RepoID: "/repo/.git", StartedAt: 1, LastSeenAt: 30, Status: session.StatusActive},
		{ID: "session-b", PaneID: "pane-b", WorkspaceRoot: "/worktree-b", RepoID: "/repo/.git", StartedAt: 1, LastSeenAt: 20, Status: session.StatusIdle},
		{ID: "session-c", PaneID: "pane-c", WorkspaceRoot: "/other", RepoID: "/other/.git", StartedAt: 1, LastSeenAt: 10, Status: session.StatusActive},
	}
	for _, value := range values {
		if err := store.Save(ctx, value); err != nil {
			t.Fatalf("Save returned error: %v", err)
		}
	}
	got, err := store.ListActiveByRepo(ctx, "/repo/.git")
	if err != nil {
		t.Fatalf("ListActiveByRepo returned error: %v", err)
	}
	if len(got) != 2 || got[0].ID != "session-a" || got[1].ID != "session-b" {
		t.Fatalf("unexpected sessions: %#v", got)
	}
}

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

func TestSessionStoreCloseStaleByWorkspace(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	store := NewSessionStore(db)
	ctx := context.Background()
	values := []session.Session{
		{ID: "session-fresh", PaneID: "pane-1", WorkspaceRoot: "/workspace", CWD: "/workspace", Branch: "main", StartedAt: 1, LastSeenAt: 100, Status: session.StatusActive},
		{ID: "session-stale", PaneID: "pane-2", WorkspaceRoot: "/workspace", CWD: "/workspace", Branch: "main", StartedAt: 1, LastSeenAt: 10, Status: session.StatusActive},
		{ID: "session-other", PaneID: "pane-3", WorkspaceRoot: "/other", CWD: "/other", Branch: "main", StartedAt: 1, LastSeenAt: 10, Status: session.StatusActive},
	}
	for _, value := range values {
		if err := store.Save(ctx, value); err != nil {
			t.Fatalf("Save returned error: %v", err)
		}
	}

	closed, err := store.CloseStaleByWorkspace(ctx, "/workspace", 50, 200)
	if err != nil {
		t.Fatalf("CloseStaleByWorkspace returned error: %v", err)
	}
	if closed != 1 {
		t.Fatalf("closed = %d, want 1", closed)
	}
	stale, err := store.FindByID(ctx, "session-stale")
	if err != nil {
		t.Fatalf("FindByID returned error: %v", err)
	}
	if stale.Status != session.StatusClosed {
		t.Fatalf("stale status = %q", stale.Status)
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
