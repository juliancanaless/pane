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

func (f *fakeStore) ListActiveByWorkspace(context.Context, string) ([]Session, error) {
	return []Session{f.status}, nil
}

func (f *fakeStore) UpdateIntent(context.Context, string, string, int64) error {
	return nil
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
