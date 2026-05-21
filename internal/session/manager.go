package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

const ResumeWindow = 4 * time.Hour

type Store interface {
	Save(context.Context, Session) error
	FindResumable(context.Context, string, string, string, int64) (Session, error)
	FindByPaneWorkspace(context.Context, string, string) (Session, error)
	FindByID(context.Context, string) (Session, error)
	ListActiveByWorkspace(context.Context, string) ([]Session, error)
	ListRecentByWorkspace(context.Context, string, int) ([]Session, error)
	UpdateIntent(context.Context, string, string, int64) error
}

type Manager struct {
	store Store
	now   func() time.Time
}

type InitInput struct {
	PaneID        string
	TTY           string
	WorkspaceRoot string
	CWD           string
	Branch        string
}

type InitResult struct {
	Session Session
	Resumed bool
}

func NewManager(store Store) Manager {
	return Manager{store: store, now: time.Now}
}

func (m Manager) Init(ctx context.Context, input InitInput) (InitResult, error) {
	now := m.now().Unix()
	seenAfter := m.now().Add(-ResumeWindow).Unix()
	current, err := m.store.FindResumable(ctx, input.PaneID, input.WorkspaceRoot, input.Branch, seenAfter)
	if err == nil {
		current.TTY = input.TTY
		current.CWD = input.CWD
		current.Branch = input.Branch
		current.LastSeenAt = now
		current.Status = StatusActive
		return InitResult{Session: current, Resumed: true}, m.store.Save(ctx, current)
	}
	if !errors.Is(err, ErrNotFound) {
		return InitResult{}, err
	}

	created := Session{
		ID:            newID(),
		PaneID:        input.PaneID,
		TTY:           input.TTY,
		WorkspaceRoot: input.WorkspaceRoot,
		CWD:           input.CWD,
		Branch:        input.Branch,
		StartedAt:     now,
		LastSeenAt:    now,
		Status:        StatusActive,
	}
	return InitResult{Session: created}, m.store.Save(ctx, created)
}

func (m Manager) Status(ctx context.Context, paneID, workspaceRoot string) (Session, error) {
	return m.store.FindByPaneWorkspace(ctx, paneID, workspaceRoot)
}

func (m Manager) Heartbeat(ctx context.Context, input InitInput) (InitResult, error) {
	now := m.now().Unix()
	current, err := m.store.FindByPaneWorkspace(ctx, input.PaneID, input.WorkspaceRoot)
	if errors.Is(err, ErrNotFound) {
		return m.Init(ctx, input)
	}
	if err != nil {
		return InitResult{}, err
	}
	current.TTY = input.TTY
	current.CWD = input.CWD
	current.Branch = input.Branch
	current.LastSeenAt = now
	current.Status = StatusActive
	return InitResult{Session: current, Resumed: true}, m.store.Save(ctx, current)
}

func (m Manager) ListActive(ctx context.Context, workspaceRoot string) ([]Session, error) {
	return m.store.ListActiveByWorkspace(ctx, workspaceRoot)
}

func (m Manager) ListRecent(ctx context.Context, workspaceRoot string, limit int) ([]Session, error) {
	return m.store.ListRecentByWorkspace(ctx, workspaceRoot, limit)
}

func (m Manager) Continue(ctx context.Context, input InitInput, parentID string) (InitResult, error) {
	parent, err := m.store.FindByID(ctx, parentID)
	if err != nil {
		return InitResult{}, err
	}
	if parent.WorkspaceRoot != input.WorkspaceRoot {
		return InitResult{}, errors.New("cannot continue a session from a different workspace")
	}

	now := m.now().Unix()
	current, err := m.store.FindByPaneWorkspace(ctx, input.PaneID, input.WorkspaceRoot)
	if err == nil && current.ID != parent.ID {
		current.TTY = input.TTY
		current.CWD = input.CWD
		current.Branch = input.Branch
		current.LastIntent = parent.LastIntent
		current.LastSeenAt = now
		current.Status = StatusActive
		current.ParentID = parent.ID
		return InitResult{Session: current, Resumed: true}, m.store.Save(ctx, current)
	}
	if err != nil && !errors.Is(err, ErrNotFound) {
		return InitResult{}, err
	}

	created := Session{
		ID:            newID(),
		PaneID:        input.PaneID,
		TTY:           input.TTY,
		WorkspaceRoot: input.WorkspaceRoot,
		CWD:           input.CWD,
		Branch:        input.Branch,
		LastIntent:    parent.LastIntent,
		StartedAt:     now,
		LastSeenAt:    now,
		Status:        StatusActive,
		ParentID:      parent.ID,
	}
	return InitResult{Session: created}, m.store.Save(ctx, created)
}

func (m Manager) SetIntent(ctx context.Context, sessionID, intent string) error {
	return m.store.UpdateIntent(ctx, sessionID, intent, m.now().Unix())
}

func newID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "session-" + hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return "session-" + hex.EncodeToString(bytes[:])
}
