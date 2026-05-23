package session

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	saved     Session
	resumable Session
	status    Session
	byID      Session
	recent    []Session
	findErr   error
}

func (f *fakeStore) Save(_ context.Context, value Session) error {
	f.saved = value
	return nil
}

func (f *fakeStore) FindResumable(context.Context, string, string, string, int64) (Session, error) {
	if f.findErr != nil {
		return Session{}, f.findErr
	}
	return f.resumable, nil
}

func (f *fakeStore) FindByPaneWorkspace(context.Context, string, string) (Session, error) {
	if f.status.ID == "" {
		return Session{}, ErrNotFound
	}
	return f.status, nil
}

func (f *fakeStore) FindByID(_ context.Context, sessionID string) (Session, error) {
	if f.byID.ID == sessionID {
		return f.byID, nil
	}
	if f.status.ID == sessionID {
		return f.status, nil
	}
	return Session{}, ErrNotFound
}

func (f *fakeStore) ListActiveByWorkspace(context.Context, string) ([]Session, error) {
	if f.recent != nil {
		return f.recent, nil
	}
	return []Session{f.status}, nil
}

func (f *fakeStore) ListRecentByWorkspace(context.Context, string, int) ([]Session, error) {
	if f.recent != nil {
		return f.recent, nil
	}
	return []Session{f.status}, nil
}

func (f *fakeStore) ListActiveByRepo(context.Context, string) ([]Session, error) {
	return f.ListActiveByWorkspace(context.Background(), "")
}

func (f *fakeStore) ListRecentByRepo(context.Context, string, int) ([]Session, error) {
	return f.ListRecentByWorkspace(context.Background(), "", 0)
}

func (f *fakeStore) UpdateIntent(context.Context, string, string, int64) error {
	return nil
}

func (f *fakeStore) UpdateStatus(_ context.Context, sessionID string, status Status, seenAt int64) error {
	if f.status.ID != "" && f.status.ID != sessionID {
		return ErrNotFound
	}
	f.status.ID = sessionID
	f.status.Status = status
	f.status.LastSeenAt = seenAt
	return nil
}

func (f *fakeStore) UpdateName(_ context.Context, sessionID, name string) error {
	if f.status.ID == sessionID {
		f.status.Name = name
		return nil
	}
	return ErrNotFound
}

func (f *fakeStore) FindByName(_ context.Context, workspaceRoot, name string) (Session, error) {
	for _, item := range f.recent {
		if item.Name == name && item.WorkspaceRoot == workspaceRoot {
			return item, nil
		}
	}
	if f.status.Name == name && f.status.WorkspaceRoot == workspaceRoot {
		return f.status, nil
	}
	return Session{}, ErrNotFound
}

func (f *fakeStore) CloseStaleByWorkspace(_ context.Context, _ string, seenBefore int64, seenAt int64) (int64, error) {
	var count int64
	for i, item := range f.recent {
		if item.Status != StatusClosed && item.LastSeenAt < seenBefore {
			f.recent[i].Status = StatusClosed
			f.recent[i].LastSeenAt = seenAt
			count++
		}
	}
	return count, nil
}

func TestManagerInitCreatesWhenNoResumableSession(t *testing.T) {
	store := &fakeStore{findErr: ErrNotFound}
	manager := NewManager(store)
	manager.now = func() time.Time { return time.Unix(1000, 0) }

	result, err := manager.Init(context.Background(), InitInput{
		PaneID:        "tty:/dev/ttys001",
		WorkspaceRoot: "/repo",
		CWD:           "/repo",
		Branch:        "main",
	})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if result.Resumed {
		t.Fatal("expected new session")
	}
	if result.Session.ID == "" {
		t.Fatal("expected session id")
	}
	if store.saved.Status != StatusActive {
		t.Fatalf("saved status = %q", store.saved.Status)
	}
}

func TestManagerInitCreatesChildSessionFromParentEnv(t *testing.T) {
	store := &fakeStore{
		findErr: ErrNotFound,
		byID:    Session{ID: "session-parent", WorkspaceRoot: "/repo", LastIntent: "split out worker task"},
	}
	manager := NewManager(store)
	manager.now = func() time.Time { return time.Unix(1000, 0) }

	result, err := manager.Init(context.Background(), InitInput{
		PaneID:          "spawn:worker",
		WorkspaceRoot:   "/repo",
		CWD:             "/repo",
		Branch:          "main",
		ParentSessionID: "session-parent",
	})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if result.Session.ParentID != "session-parent" {
		t.Fatalf("parent id = %q", result.Session.ParentID)
	}
	if result.Session.LastIntent != "split out worker task" {
		t.Fatalf("intent = %q", result.Session.LastIntent)
	}
}

func TestManagerInitRejectsChildFromDifferentWorkspace(t *testing.T) {
	store := &fakeStore{
		findErr: ErrNotFound,
		byID:    Session{ID: "session-parent", WorkspaceRoot: "/other"},
	}
	manager := NewManager(store)

	_, err := manager.Init(context.Background(), InitInput{WorkspaceRoot: "/repo", ParentSessionID: "session-parent"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestManagerInitResumesExistingSession(t *testing.T) {
	store := &fakeStore{resumable: Session{ID: "session-existing", Status: StatusIdle}}
	manager := NewManager(store)
	manager.now = func() time.Time { return time.Unix(1000, 0) }

	result, err := manager.Init(context.Background(), InitInput{
		PaneID:        "tty:/dev/ttys001",
		WorkspaceRoot: "/repo",
		CWD:           "/repo/subdir",
		Branch:        "main",
	})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if !result.Resumed {
		t.Fatal("expected resumed session")
	}
	if result.Session.ID != "session-existing" {
		t.Fatalf("session id = %q", result.Session.ID)
	}
	if result.Session.Status != StatusActive {
		t.Fatalf("status = %q", result.Session.Status)
	}
}

func TestManagerInitReturnsStoreErrors(t *testing.T) {
	store := &fakeStore{findErr: errors.New("database unavailable")}
	manager := NewManager(store)

	_, err := manager.Init(context.Background(), InitInput{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestManagerHeartbeatRefreshesExistingSession(t *testing.T) {
	store := &fakeStore{status: Session{ID: "session-existing", WorkspaceRoot: "/repo", LastIntent: "keep intent", Status: StatusIdle}}
	manager := NewManager(store)
	manager.now = func() time.Time { return time.Unix(1000, 0) }

	result, err := manager.Heartbeat(context.Background(), InitInput{PaneID: "pane-1", TTY: "/dev/ttys001", WorkspaceRoot: "/repo", CWD: "/repo/subdir", Branch: "feature"})
	if err != nil {
		t.Fatalf("Heartbeat returned error: %v", err)
	}
	if !result.Resumed {
		t.Fatal("expected existing session to be refreshed")
	}
	if result.Session.LastIntent != "keep intent" {
		t.Fatalf("intent = %q", result.Session.LastIntent)
	}
	if result.Session.CWD != "/repo/subdir" || result.Session.Branch != "feature" || result.Session.Status != StatusActive {
		t.Fatalf("unexpected refreshed session: %#v", result.Session)
	}
}

func TestManagerResolveMatchesShortID(t *testing.T) {
	store := &fakeStore{recent: []Session{{ID: "session-abcdef1234567890", WorkspaceRoot: "/repo"}}}
	manager := NewManager(store)

	result, err := manager.Resolve(context.Background(), "/repo", "abcdef12")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if result.ID != "session-abcdef1234567890" {
		t.Fatalf("session id = %q", result.ID)
	}
}

func TestManagerResolveReturnsAmbiguousForSharedPrefix(t *testing.T) {
	store := &fakeStore{recent: []Session{{ID: "session-abcdef1234567890", WorkspaceRoot: "/repo"}, {ID: "session-abcdef9999999999", WorkspaceRoot: "/repo"}}}
	manager := NewManager(store)

	_, err := manager.Resolve(context.Background(), "/repo", "abc")
	if !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("Resolve error = %v", err)
	}
}

func TestManagerResolveByName(t *testing.T) {
	store := &fakeStore{recent: []Session{
		{ID: "session-aaa", WorkspaceRoot: "/repo", Name: "auth-refactor"},
		{ID: "session-bbb", WorkspaceRoot: "/repo", Name: "db-migration"},
	}}
	manager := NewManager(store)

	// Exact name match via FindByName
	result, err := manager.Resolve(context.Background(), "/repo", "auth-refactor")
	if err != nil {
		t.Fatalf("Resolve by name returned error: %v", err)
	}
	if result.ID != "session-aaa" {
		t.Fatalf("session id = %q, want session-aaa", result.ID)
	}

	// Name prefix match
	result, err = manager.Resolve(context.Background(), "/repo", "db-")
	if err != nil {
		t.Fatalf("Resolve by name prefix returned error: %v", err)
	}
	if result.ID != "session-bbb" {
		t.Fatalf("session id = %q, want session-bbb", result.ID)
	}
}

func TestManagerSetName(t *testing.T) {
	store := &fakeStore{status: Session{ID: "session-test", WorkspaceRoot: "/repo", Status: StatusActive}}
	manager := NewManager(store)

	err := manager.SetName(context.Background(), "session-test", "my-task")
	if err != nil {
		t.Fatalf("SetName returned error: %v", err)
	}
	if store.status.Name != "my-task" {
		t.Fatalf("name = %q, want my-task", store.status.Name)
	}
}

func TestManagerListActiveFiltersStaleSessions(t *testing.T) {
	store := &fakeStore{recent: []Session{{ID: "fresh", LastSeenAt: 990, Status: StatusActive}, {ID: "stale", LastSeenAt: 100, Status: StatusActive}}}
	manager := NewManager(store)
	manager.now = func() time.Time { return time.Unix(2000, 0) }

	items, err := manager.ListActive(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("ListActive returned error: %v", err)
	}
	if len(items) != 1 || items[0].ID != "fresh" {
		t.Fatalf("items = %#v", items)
	}
}

func TestManagerCloseMarksCurrentSessionClosed(t *testing.T) {
	store := &fakeStore{status: Session{ID: "session-current", WorkspaceRoot: "/repo", Status: StatusActive}}
	manager := NewManager(store)
	manager.now = func() time.Time { return time.Unix(1000, 0) }

	closed, err := manager.Close(context.Background(), "pane-1", "/repo")
	if err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if closed.Status != StatusClosed || store.status.Status != StatusClosed {
		t.Fatalf("session was not closed: result=%#v store=%#v", closed, store.status)
	}
}

func TestManagerContinueLinksCurrentPaneToParent(t *testing.T) {
	store := &fakeStore{status: Session{ID: "session-parent", WorkspaceRoot: "/repo", LastIntent: "finish docs"}}
	manager := NewManager(store)
	manager.now = func() time.Time { return time.Unix(1000, 0) }

	result, err := manager.Continue(context.Background(), InitInput{PaneID: "pane-2", WorkspaceRoot: "/repo", CWD: "/repo", Branch: "main"}, "session-parent")
	if err != nil {
		t.Fatalf("Continue returned error: %v", err)
	}
	if result.Session.ParentID != "session-parent" {
		t.Fatalf("parent id = %q", result.Session.ParentID)
	}
	if result.Session.LastIntent != "finish docs" {
		t.Fatalf("intent = %q", result.Session.LastIntent)
	}
}

func TestManagerContinueOverwritesExistingPaneIntentWithParentIntent(t *testing.T) {
	store := &fakeStore{
		status: Session{ID: "session-current", WorkspaceRoot: "/repo", LastIntent: "old current intent"},
		byID:   Session{ID: "session-parent", WorkspaceRoot: "/repo", LastIntent: "parent handoff intent"},
	}
	manager := NewManager(store)
	manager.now = func() time.Time { return time.Unix(1000, 0) }

	result, err := manager.Continue(context.Background(), InitInput{PaneID: "pane-2", WorkspaceRoot: "/repo", CWD: "/repo", Branch: "main"}, "session-parent")
	if err != nil {
		t.Fatalf("Continue returned error: %v", err)
	}
	if result.Session.LastIntent != "parent handoff intent" {
		t.Fatalf("intent = %q", result.Session.LastIntent)
	}
}
