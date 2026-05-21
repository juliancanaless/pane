package store

import (
	"context"
	"database/sql"
	"errors"

	"github.com/juliancanalez/pane/internal/session"
)

type SessionStore struct {
	db *sql.DB
}

func NewSessionStore(db *sql.DB) SessionStore {
	return SessionStore{db: db}
}

const sessionColumns = `session_id, pane_id, tty, workspace_root, cwd, branch, last_intent, started_at, last_seen_at, status, parent_session_id, COALESCE(name, '')`

func (s SessionStore) Save(ctx context.Context, value session.Session) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO sessions (
    session_id, pane_id, tty, workspace_root, cwd, branch, last_intent, started_at, last_seen_at, status, parent_session_id, name
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(session_id) DO UPDATE SET
    pane_id = excluded.pane_id,
    tty = excluded.tty,
    workspace_root = excluded.workspace_root,
    cwd = excluded.cwd,
    branch = excluded.branch,
    last_intent = excluded.last_intent,
    last_seen_at = excluded.last_seen_at,
    status = excluded.status,
    parent_session_id = excluded.parent_session_id,
    name = COALESCE(excluded.name, sessions.name)
`, value.ID, value.PaneID, value.TTY, value.WorkspaceRoot, value.CWD, value.Branch, value.LastIntent, value.StartedAt, value.LastSeenAt, string(value.Status), nullableString(value.ParentID), nullableString(value.Name))
	return err
}

func (s SessionStore) FindResumable(ctx context.Context, paneID, workspaceRoot, branch string, seenAfter int64) (session.Session, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT `+sessionColumns+`
FROM sessions
WHERE pane_id = ?
  AND workspace_root = ?
  AND branch = ?
  AND status IN ('active', 'idle')
  AND last_seen_at >= ?
ORDER BY last_seen_at DESC
LIMIT 1
`, paneID, workspaceRoot, branch, seenAfter)
	return scanSession(row)
}

func (s SessionStore) FindByPaneWorkspace(ctx context.Context, paneID, workspaceRoot string) (session.Session, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT `+sessionColumns+`
FROM sessions
WHERE pane_id = ?
  AND workspace_root = ?
  AND status IN ('active', 'idle')
ORDER BY last_seen_at DESC
LIMIT 1
`, paneID, workspaceRoot)
	return scanSession(row)
}

func (s SessionStore) FindByID(ctx context.Context, sessionID string) (session.Session, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT `+sessionColumns+`
FROM sessions
WHERE session_id = ?
LIMIT 1
`, sessionID)
	return scanSession(row)
}

func (s SessionStore) ListActiveByWorkspace(ctx context.Context, workspaceRoot string) ([]session.Session, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT `+sessionColumns+`
FROM sessions
WHERE workspace_root = ?
  AND status IN ('active', 'idle')
ORDER BY last_seen_at DESC
`, workspaceRoot)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessions(rows)
}

func (s SessionStore) ListRecentByWorkspace(ctx context.Context, workspaceRoot string, limit int) ([]session.Session, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT `+sessionColumns+`
FROM sessions
WHERE workspace_root = ?
ORDER BY last_seen_at DESC
LIMIT ?
`, workspaceRoot, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSessions(rows)
}

func (s SessionStore) UpdateStatus(ctx context.Context, sessionID string, status session.Status, seenAt int64) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE sessions
SET status = ?, last_seen_at = ?
WHERE session_id = ?
`, string(status), seenAt, sessionID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return session.ErrNotFound
	}
	return nil
}

func (s SessionStore) CloseStaleByWorkspace(ctx context.Context, workspaceRoot string, seenBefore int64, seenAt int64) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
UPDATE sessions
SET status = 'closed', last_seen_at = ?
WHERE workspace_root = ?
  AND status IN ('active', 'idle')
  AND last_seen_at < ?
`, seenAt, workspaceRoot, seenBefore)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s SessionStore) UpdateIntent(ctx context.Context, sessionID, intent string, seenAt int64) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE sessions
SET last_intent = ?, last_seen_at = ?, status = 'active'
WHERE session_id = ?
`, intent, seenAt, sessionID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return session.ErrNotFound
	}
	return nil
}

type scanner interface {
	Scan(dest ...any) error
}

func (s SessionStore) UpdateName(ctx context.Context, sessionID, name string) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE sessions SET name = ? WHERE session_id = ?
`, nullableString(name), sessionID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return session.ErrNotFound
	}
	return nil
}

func (s SessionStore) FindByName(ctx context.Context, workspaceRoot, name string) (session.Session, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT `+sessionColumns+`
FROM sessions
WHERE workspace_root = ?
  AND name = ? COLLATE NOCASE
  AND status IN ('active', 'idle')
ORDER BY last_seen_at DESC
LIMIT 1
`, workspaceRoot, name)
	return scanSession(row)
}

func scanSession(row scanner) (session.Session, error) {
	var value session.Session
	var status string
	var parent sql.NullString
	err := row.Scan(
		&value.ID,
		&value.PaneID,
		&value.TTY,
		&value.WorkspaceRoot,
		&value.CWD,
		&value.Branch,
		&value.LastIntent,
		&value.StartedAt,
		&value.LastSeenAt,
		&status,
		&parent,
		&value.Name,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return session.Session{}, session.ErrNotFound
	}
	if err != nil {
		return session.Session{}, err
	}
	value.Status = session.Status(status)
	value.ParentID = parent.String
	return value, nil
}

func scanSessions(rows *sql.Rows) ([]session.Session, error) {
	var sessions []session.Session
	for rows.Next() {
		value, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return sessions, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
