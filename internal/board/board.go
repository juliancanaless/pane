package board

import "github.com/juliancanalez/pane/internal/session"

type Board struct {
	WorkspaceRoot string
	Sessions      []Session
}

type Session struct {
	ID         string
	Status     session.Status
	Branch     string
	CWD        string
	LastIntent string
	LastSeenAt int64
}

func FromSessions(workspaceRoot string, sessions []session.Session) Board {
	items := make([]Session, 0, len(sessions))
	for _, value := range sessions {
		items = append(items, Session{
			ID:         value.ID,
			Status:     value.Status,
			Branch:     value.Branch,
			CWD:        value.CWD,
			LastIntent: value.LastIntent,
			LastSeenAt: value.LastSeenAt,
		})
	}
	return Board{WorkspaceRoot: workspaceRoot, Sessions: items}
}
