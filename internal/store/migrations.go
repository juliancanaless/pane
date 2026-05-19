package store

import "database/sql"

func Migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
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
    status         TEXT NOT NULL DEFAULT 'active'
);

CREATE INDEX IF NOT EXISTS idx_sessions_lookup ON sessions(pane_id, workspace_root, status);

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
`
