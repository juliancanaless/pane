package summary

import "github.com/juliancanalez/pane/internal/session"

type StartupSummary struct {
	WorkspaceRoot string
	Current       SessionLine
	Peers         []SessionLine
}

type SessionLine struct {
	SessionID  string
	Status     session.Status
	Branch     string
	CWD        string
	LastIntent string
	LastSeenAt int64
}

func FromSessions(workspaceRoot string, current session.Session, sessions []session.Session) StartupSummary {
	peers := make([]SessionLine, 0, len(sessions))
	for _, value := range sessions {
		line := fromSession(value)
		if value.ID == current.ID {
			continue
		}
		peers = append(peers, line)
	}
	return StartupSummary{WorkspaceRoot: workspaceRoot, Current: fromSession(current), Peers: peers}
}

func fromSession(value session.Session) SessionLine {
	return SessionLine{
		SessionID:  value.ID,
		Status:     value.Status,
		Branch:     value.Branch,
		CWD:        value.CWD,
		LastIntent: value.LastIntent,
		LastSeenAt: value.LastSeenAt,
	}
}
