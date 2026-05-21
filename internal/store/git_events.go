package store

import (
	"context"
	"database/sql"
)

type GitEvent struct {
	ID           int64
	SessionID    string
	Command      string
	Subcommand   string
	Branch       string
	TargetBranch string
	Timestamp    int64
	Result       string
}

type GitEventStore struct {
	db *sql.DB
}

func NewGitEventStore(db *sql.DB) GitEventStore {
	return GitEventStore{db: db}
}

func (s GitEventStore) Save(ctx context.Context, value GitEvent) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO git_events (session_id, command, subcommand, branch, target_branch, timestamp, result)
VALUES (?, ?, ?, ?, ?, ?, ?)
`, value.SessionID, value.Command, value.Subcommand, value.Branch, value.TargetBranch, value.Timestamp, value.Result)
	return err
}

func (s GitEventStore) RecentByBranch(ctx context.Context, branch string, since int64, limit int) ([]GitEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, session_id, command, subcommand, branch, COALESCE(target_branch, ''), timestamp, COALESCE(result, '')
FROM git_events
WHERE branch = ?
  AND timestamp >= ?
ORDER BY timestamp DESC, id DESC
LIMIT ?
`, branch, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []GitEvent
	for rows.Next() {
		var value GitEvent
		if err := rows.Scan(&value.ID, &value.SessionID, &value.Command, &value.Subcommand, &value.Branch, &value.TargetBranch, &value.Timestamp, &value.Result); err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s GitEventStore) RecentByWorkspace(ctx context.Context, workspaceRoot string, since int64, limit int) ([]GitEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT ge.id, ge.session_id, ge.command, ge.subcommand, ge.branch, COALESCE(ge.target_branch, ''), ge.timestamp, COALESCE(ge.result, '')
FROM git_events ge
JOIN sessions s ON s.session_id = ge.session_id
WHERE s.workspace_root = ?
  AND ge.timestamp >= ?
ORDER BY ge.timestamp DESC, ge.id DESC
LIMIT ?
`, workspaceRoot, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []GitEvent
	for rows.Next() {
		var value GitEvent
		if err := rows.Scan(&value.ID, &value.SessionID, &value.Command, &value.Subcommand, &value.Branch, &value.TargetBranch, &value.Timestamp, &value.Result); err != nil {
			return nil, err
		}
		items = append(items, value)
	}
	return items, rows.Err()
}
