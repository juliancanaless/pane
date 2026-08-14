package store

import (
	"database/sql"
	"strings"
)

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	for _, statement := range additiveMigrations {
		if _, err := db.Exec(statement); err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return err
		}
	}
	return nil
}

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
    session_id     TEXT PRIMARY KEY,
    pane_id        TEXT NOT NULL,
    tty            TEXT,
    workspace_root TEXT NOT NULL,
    cwd            TEXT,
    branch         TEXT,
    last_intent    TEXT,
    started_at     INTEGER NOT NULL,
    last_seen_at   INTEGER NOT NULL,
    status         TEXT NOT NULL DEFAULT 'active',
    parent_session_id TEXT REFERENCES sessions(session_id)
);

CREATE INDEX IF NOT EXISTS idx_sessions_lookup ON sessions(pane_id, workspace_root, status);
CREATE INDEX IF NOT EXISTS idx_sessions_workspace_seen ON sessions(workspace_root, last_seen_at);

CREATE TABLE IF NOT EXISTS file_activity (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id    TEXT NOT NULL REFERENCES sessions(session_id),
    path          TEXT NOT NULL,
    event_type    TEXT NOT NULL,
    attribution   TEXT NOT NULL DEFAULT 'high',
    timestamp     INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_file_activity_session ON file_activity(session_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_file_activity_path ON file_activity(path, timestamp);

CREATE TABLE IF NOT EXISTS git_events (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id    TEXT NOT NULL REFERENCES sessions(session_id),
    command       TEXT NOT NULL,
    subcommand    TEXT NOT NULL,
    branch        TEXT,
    target_branch TEXT,
    timestamp     INTEGER NOT NULL,
    result        TEXT
);

CREATE INDEX IF NOT EXISTS idx_git_events_session ON git_events(session_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_git_events_branch ON git_events(branch, timestamp);

CREATE TABLE IF NOT EXISTS messages (
    message_id    TEXT PRIMARY KEY,
    thread_id     TEXT NOT NULL,
    from_session  TEXT NOT NULL REFERENCES sessions(session_id),
    to_session    TEXT NOT NULL,
    body          TEXT NOT NULL,
    state         TEXT NOT NULL DEFAULT 'queued',
    created_at    INTEGER NOT NULL,
    delivered_at  INTEGER
);

CREATE INDEX IF NOT EXISTS idx_messages_to ON messages(to_session, state);
CREATE INDEX IF NOT EXISTS idx_messages_thread ON messages(thread_id, created_at);

CREATE TABLE IF NOT EXISTS agent_state (
    workspace_root TEXT NOT NULL,
    key            TEXT NOT NULL,
    value_json     TEXT NOT NULL,
    updated_at     INTEGER NOT NULL,
    session_id     TEXT REFERENCES sessions(session_id),
    PRIMARY KEY (workspace_root, key)
);

CREATE INDEX IF NOT EXISTS idx_agent_state_workspace_key ON agent_state(workspace_root, key);

CREATE TABLE IF NOT EXISTS analysis_symbols (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_root TEXT NOT NULL,
    file           TEXT NOT NULL,
    language       TEXT NOT NULL,
    name           TEXT NOT NULL,
    kind           TEXT NOT NULL,
    start_line     INTEGER NOT NULL,
    end_line       INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_analysis_symbols_lookup ON analysis_symbols(workspace_root, file, name);
CREATE INDEX IF NOT EXISTS idx_analysis_symbols_name ON analysis_symbols(workspace_root, name);

CREATE TABLE IF NOT EXISTS dependency_edges (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_root TEXT NOT NULL,
    source_file    TEXT NOT NULL,
    target         TEXT NOT NULL,
    target_symbol  TEXT NOT NULL DEFAULT '',
    kind           TEXT NOT NULL,
    confidence     REAL NOT NULL DEFAULT 0.5,
    updated_at     INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_dependency_edges_source ON dependency_edges(workspace_root, source_file);
CREATE INDEX IF NOT EXISTS idx_dependency_edges_target ON dependency_edges(workspace_root, target, target_symbol);
`

var additiveMigrations = []string{
	`ALTER TABLE sessions ADD COLUMN parent_session_id TEXT REFERENCES sessions(session_id)`,
	`CREATE INDEX IF NOT EXISTS idx_sessions_parent ON sessions(parent_session_id)`,
	`ALTER TABLE sessions ADD COLUMN name TEXT`,
	`ALTER TABLE sessions ADD COLUMN repo_id TEXT`,
	`ALTER TABLE sessions ADD COLUMN git_common_dir TEXT`,
	`CREATE INDEX IF NOT EXISTS idx_sessions_repo_seen ON sessions(repo_id, last_seen_at)`,
	`ALTER TABLE sessions ADD COLUMN agent_session_id TEXT`,
	`CREATE INDEX IF NOT EXISTS idx_sessions_agent ON sessions(agent_session_id, last_seen_at)`,
}
