package session

import "errors"

var ErrNotFound = errors.New("not found")

type Status string

const (
	StatusActive Status = "active"
	StatusIdle   Status = "idle"
	StatusClosed Status = "closed"
)

type Session struct {
	ID            string
	PaneID        string
	TTY           string
	WorkspaceRoot string
	CWD           string
	Branch        string
	LastIntent    string
	StartedAt     int64
	LastSeenAt    int64
	Status        Status
	ParentID      string
}
