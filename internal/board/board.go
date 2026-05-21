package board

import "github.com/juliancanalez/pane/internal/session"

type Board struct {
	WorkspaceRoot string
	Sessions      []Session
}

type Session struct {
	ID              string
	Status          session.Status
	Branch          string
	CWD             string
	LastIntent      string
	LastSeenAt      int64
	UnreadMessages  int
	AwaitingReplies int
}

func FromSessions(workspaceRoot string, sessions []session.Session) Board {
	return FromSessionsWithMessages(workspaceRoot, sessions, nil)
}

func FromSessionsWithMessages(workspaceRoot string, sessions []session.Session, stats map[string]MessageStats) Board {
	items := make([]Session, 0, len(sessions))
	for _, value := range sessions {
		messageStats := stats[value.ID]
		items = append(items, Session{
			ID:              value.ID,
			Status:          value.Status,
			Branch:          value.Branch,
			CWD:             value.CWD,
			LastIntent:      value.LastIntent,
			LastSeenAt:      value.LastSeenAt,
			UnreadMessages:  messageStats.UnreadMessages,
			AwaitingReplies: messageStats.AwaitingReplies,
		})
	}
	return Board{WorkspaceRoot: workspaceRoot, Sessions: items}
}

type MessageStats struct {
	UnreadMessages  int
	AwaitingReplies int
}
