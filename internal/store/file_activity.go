package store

import (
	"context"
	"database/sql"

	"github.com/juliancanalez/pane/internal/activity"
)

type FileActivityStore struct {
	db *sql.DB
}

func NewFileActivityStore(db *sql.DB) FileActivityStore {
	return FileActivityStore{db: db}
}

func (s FileActivityStore) Save(ctx context.Context, value activity.FileActivity) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO file_activity (session_id, path, event_type, attribution, timestamp)
VALUES (?, ?, ?, ?, ?)
`, value.SessionID, value.Path, string(value.EventType), string(value.Attribution), value.Timestamp)
	return err
}

func (s FileActivityStore) RecentBySession(ctx context.Context, sessionID string, since int64, limit int) ([]activity.FileActivity, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, session_id, path, event_type, attribution, timestamp
FROM file_activity
WHERE session_id = ?
  AND timestamp >= ?
ORDER BY timestamp DESC, id DESC
LIMIT ?
`, sessionID, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFileActivities(rows)
}

func (s FileActivityStore) RecentByWorkspace(ctx context.Context, workspaceRoot string, since int64, limit int) ([]activity.FileActivity, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT fa.id, fa.session_id, fa.path, fa.event_type, fa.attribution, fa.timestamp
FROM file_activity fa
JOIN sessions s ON s.session_id = fa.session_id
WHERE s.workspace_root = ?
  AND fa.timestamp >= ?
ORDER BY fa.timestamp DESC, fa.id DESC
LIMIT ?
`, workspaceRoot, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFileActivities(rows)
}

type fileActivityRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}

func scanFileActivities(rows fileActivityRows) ([]activity.FileActivity, error) {
	var items []activity.FileActivity
	for rows.Next() {
		var value activity.FileActivity
		var eventType string
		var attribution string
		if err := rows.Scan(&value.ID, &value.SessionID, &value.Path, &eventType, &attribution, &value.Timestamp); err != nil {
			return nil, err
		}
		value.EventType = activity.EventType(eventType)
		value.Attribution = activity.Attribution(attribution)
		items = append(items, value)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
