package summary

import (
	"github.com/juliancanalez/pane/internal/messages"
	"github.com/juliancanalez/pane/internal/session"
)

type StartupSummary struct {
	WorkspaceRoot string
	Current       SessionLine
	Peers         []SessionLine
	Coordination  Coordination
	RecentFiles   []string
	Lineage       Lineage
	Overlaps      []OverlapInfo
}

type OverlapInfo struct {
	PeerSessionID string
	SharedFiles   []string
}

func HistoryFromSessions(sessions []session.Session) []SessionLine {
	lines := make([]SessionLine, 0, len(sessions))
	for _, value := range sessions {
		lines = append(lines, fromSession(value))
	}
	return lines
}

type SessionLine struct {
	SessionID  string
	Status     session.Status
	Branch     string
	CWD        string
	LastIntent string
	LastSeenAt int64
	ParentID   string
}

type Lineage struct {
	Parent  *SessionLine
	History []SessionLine
}

type Coordination struct {
	UnreadMessages  []messages.Message
	AwaitingReplies int
}

func FromSessions(workspaceRoot string, current session.Session, sessions []session.Session) StartupSummary {
	return FromSessionsWithCoordination(workspaceRoot, current, sessions, Coordination{})
}

func FromSessionsWithCoordination(workspaceRoot string, current session.Session, sessions []session.Session, coordination Coordination) StartupSummary {
	return FromSessionsWithContext(workspaceRoot, current, sessions, coordination, nil)
}

func FromSessionsWithContext(workspaceRoot string, current session.Session, sessions []session.Session, coordination Coordination, recentFiles []string) StartupSummary {
	peers := make([]SessionLine, 0, len(sessions))
	for _, value := range sessions {
		line := fromSession(value)
		if value.ID == current.ID {
			continue
		}
		peers = append(peers, line)
	}
	return StartupSummary{WorkspaceRoot: workspaceRoot, Current: fromSession(current), Peers: peers, Coordination: coordination, RecentFiles: recentFiles}
}

func FromSessionsWithLineage(workspaceRoot string, current session.Session, sessions []session.Session, coordination Coordination, recentFiles []string, lineage Lineage) StartupSummary {
	value := FromSessionsWithContext(workspaceRoot, current, sessions, coordination, recentFiles)
	value.Lineage = lineage
	return value
}

func fromSession(value session.Session) SessionLine {
	return SessionLine{
		SessionID:  value.ID,
		Status:     value.Status,
		Branch:     value.Branch,
		CWD:        value.CWD,
		LastIntent: value.LastIntent,
		LastSeenAt: value.LastSeenAt,
		ParentID:   value.ParentID,
	}
}
