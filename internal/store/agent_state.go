package store

import (
	"context"
	"database/sql"
	"errors"
)

const GlobalWorkspaceRoot = "__global__"

type AgentState struct {
	WorkspaceRoot string
	Key           string
	ValueJSON     string
	UpdatedAt     int64
	SessionID     string
}

type AgentStateStore struct {
	db *sql.DB
}

func NewAgentStateStore(db *sql.DB) AgentStateStore {
	return AgentStateStore{db: db}
}

func (s AgentStateStore) Set(ctx context.Context, value AgentState) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO agent_state (workspace_root, key, value_json, updated_at, session_id)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(workspace_root, key) DO UPDATE SET
    value_json = excluded.value_json,
    updated_at = excluded.updated_at,
    session_id = excluded.session_id
`, value.WorkspaceRoot, value.Key, value.ValueJSON, value.UpdatedAt, nullableString(value.SessionID))
	return err
}

func (s AgentStateStore) Get(ctx context.Context, workspaceRoot, key string) (AgentState, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT workspace_root, key, value_json, updated_at, session_id
FROM agent_state
WHERE workspace_root = ? AND key = ?
`, workspaceRoot, key)
	return scanAgentState(row)
}

func (s AgentStateStore) List(ctx context.Context, workspaceRoot, prefix string) ([]AgentState, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT workspace_root, key, value_json, updated_at, session_id
FROM agent_state
WHERE workspace_root = ? AND key LIKE ?
ORDER BY key ASC
`, workspaceRoot, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AgentState{}
	for rows.Next() {
		item, err := scanAgentState(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (s AgentStateStore) Delete(ctx context.Context, workspaceRoot, key string) error {
	result, err := s.db.ExecContext(ctx, `
DELETE FROM agent_state
WHERE workspace_root = ? AND key = ?
`, workspaceRoot, key)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func scanAgentState(row scanner) (AgentState, error) {
	var value AgentState
	var sessionID sql.NullString
	err := row.Scan(&value.WorkspaceRoot, &value.Key, &value.ValueJSON, &value.UpdatedAt, &sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return AgentState{}, ErrNotFound
	}
	if err != nil {
		return AgentState{}, err
	}
	value.SessionID = sessionID.String
	return value, nil
}
