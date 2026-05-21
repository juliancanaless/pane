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
SELECT id, session_id, command, subcommand, branch, target_branch, timestamp, result
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
