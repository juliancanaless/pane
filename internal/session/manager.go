package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const ResumeWindow = 4 * time.Hour
const StaleAfter = 30 * time.Minute

type Store interface {
	Save(context.Context, Session) error
	FindResumable(context.Context, string, string, string, int64) (Session, error)
	FindByPaneWorkspace(context.Context, string, string) (Session, error)
	FindByID(context.Context, string) (Session, error)
	FindByName(context.Context, string, string) (Session, error)
	ListActiveByWorkspace(context.Context, string) ([]Session, error)
	ListRecentByWorkspace(context.Context, string, int) ([]Session, error)
	ListActiveByRepo(context.Context, string) ([]Session, error)
	ListRecentByRepo(context.Context, string, int) ([]Session, error)
	UpdateIntent(context.Context, string, string, int64) error
	UpdateStatus(context.Context, string, Status, int64) error
	UpdateName(context.Context, string, string) error
	CloseStaleByWorkspace(context.Context, string, int64, int64) (int64, error)
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
	RepoID        string
	GitCommonDir  string
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
		current.RepoID = input.RepoID
		current.GitCommonDir = input.GitCommonDir
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
		RepoID:        input.RepoID,
		GitCommonDir:  input.GitCommonDir,
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
	current.RepoID = input.RepoID
	current.GitCommonDir = input.GitCommonDir
	current.LastSeenAt = now
	current.Status = StatusActive
	return InitResult{Session: current, Resumed: true}, m.store.Save(ctx, current)
}

func (m Manager) ListActive(ctx context.Context, workspaceRoot string) ([]Session, error) {
	items, err := m.store.ListActiveByWorkspace(ctx, workspaceRoot)
	if err != nil {
		return nil, err
	}
	return filterFresh(items, m.now().Add(-StaleAfter).Unix()), nil
}

func (m Manager) ListRecent(ctx context.Context, workspaceRoot string, limit int) ([]Session, error) {
	return m.store.ListRecentByWorkspace(ctx, workspaceRoot, limit)
}

func (m Manager) ListActiveByRepo(ctx context.Context, repoID string) ([]Session, error) {
	items, err := m.store.ListActiveByRepo(ctx, repoID)
	if err != nil {
		return nil, err
	}
	return filterFresh(items, m.now().Add(-StaleAfter).Unix()), nil
}

func (m Manager) ListRecentByRepo(ctx context.Context, repoID string, limit int) ([]Session, error) {
	return m.store.ListRecentByRepo(ctx, repoID, limit)
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
		current.RepoID = input.RepoID
		current.GitCommonDir = input.GitCommonDir
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
		RepoID:        input.RepoID,
		GitCommonDir:  input.GitCommonDir,
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

func (m Manager) SetName(ctx context.Context, sessionID, name string) error {
	return m.store.UpdateName(ctx, sessionID, name)
}

func (m Manager) Close(ctx context.Context, paneID, workspaceRoot string) (Session, error) {
	current, err := m.store.FindByPaneWorkspace(ctx, paneID, workspaceRoot)
	if err != nil {
		return Session{}, err
	}
	now := m.now().Unix()
	if err := m.store.UpdateStatus(ctx, current.ID, StatusClosed, now); err != nil {
		return Session{}, err
	}
	current.Status = StatusClosed
	current.LastSeenAt = now
	return current, nil
}

func (m Manager) PruneStale(ctx context.Context, workspaceRoot string) (int64, error) {
	now := m.now().Unix()
	return m.store.CloseStaleByWorkspace(ctx, workspaceRoot, m.now().Add(-StaleAfter).Unix(), now)
}

func filterFresh(items []Session, seenAfter int64) []Session {
	fresh := make([]Session, 0, len(items))
	for _, item := range items {
		if item.LastSeenAt >= seenAfter {
			fresh = append(fresh, item)
		}
	}
	return fresh
}

var ErrAmbiguous = errors.New("ambiguous session reference")

func (m Manager) Resolve(ctx context.Context, workspaceRoot, reference string) (Session, error) {
	reference = strings.TrimSpace(reference)
	if reference == "" {
		return Session{}, ErrNotFound
	}
	// Exact ID match
	if exact, err := m.store.FindByID(ctx, reference); err == nil && exact.WorkspaceRoot == workspaceRoot {
		return exact, nil
	}
	// Exact name match (active/idle sessions only)
	if named, err := m.store.FindByName(ctx, workspaceRoot, reference); err == nil {
		return named, nil
	}
	// Short ID / prefix match
	items, err := m.store.ListRecentByWorkspace(ctx, workspaceRoot, 100)
	if err != nil {
		return Session{}, err
	}
	matches := make([]Session, 0, 1)
	for _, item := range items {
		if matchesSessionReference(item.ID, reference) {
			matches = append(matches, item)
		}
	}
	// Also try case-insensitive name prefix match
	refLower := strings.ToLower(reference)
	for _, item := range items {
		if item.Name != "" && strings.HasPrefix(strings.ToLower(item.Name), refLower) {
			// Avoid duplicates if already matched by ID
			alreadyMatched := false
			for _, m := range matches {
				if m.ID == item.ID {
					alreadyMatched = true
					break
				}
			}
			if !alreadyMatched {
				matches = append(matches, item)
			}
		}
	}
	if len(matches) == 0 {
		return Session{}, ErrNotFound
	}
	if len(matches) > 1 {
		return Session{}, fmt.Errorf("%w: %q matches %d sessions", ErrAmbiguous, reference, len(matches))
	}
	return matches[0], nil
}

func matchesSessionReference(sessionID, reference string) bool {
	return strings.HasPrefix(sessionID, reference) || strings.HasPrefix(ShortID(sessionID), reference) || strings.HasPrefix(strings.TrimPrefix(sessionID, "session-"), strings.TrimPrefix(reference, "session-"))
}

func newID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "session-" + hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return "session-" + hex.EncodeToString(bytes[:])
}
