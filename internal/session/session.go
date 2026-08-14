package session

import (
	"errors"
	"strings"
)

var ErrNotFound = errors.New("not found")

type Status string

const (
	StatusActive Status = "active"
	StatusIdle   Status = "idle"
	StatusClosed Status = "closed"
)

func ShortID(sessionID string) string {
	short := strings.TrimPrefix(sessionID, "session-")
	if len(short) > 8 {
		return short[:8]
	}
	return short
}

type Session struct {
	ID            string
	PaneID        string
	TTY           string
	WorkspaceRoot string
	CWD           string
	Branch        string
	RepoID        string
	GitCommonDir  string
	LastIntent    string
	StartedAt     int64
	LastSeenAt    int64
	Status        Status
	ParentID      string
	Name          string
	// AgentSessionID binds this pane session to a host agent's own session id
	// (for example the Claude Code session_id delivered to hooks), so hook and
	// statusline processes without a usable tty can still find their session.
	AgentSessionID string
}
