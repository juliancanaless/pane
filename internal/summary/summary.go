package summary

import (
	"github.com/juliancanalez/pane/internal/messages"
	"github.com/juliancanalez/pane/internal/session"
)

type StartupSummary struct {
	WorkspaceRoot     string
	Current           SessionLine
	Peers             []SessionLine
	Coordination      Coordination
	RecentFiles       []string
	ActivitySummaries []string
	StateItems        []StateItem
	Lineage           Lineage
	Overlaps          []OverlapInfo
	SemanticOverlaps  []SemanticOverlapInfo
}

type StateItem struct {
	Key       string
	ValueJSON string
	SessionID string
	UpdatedAt int64
}

type OverlapInfo struct {
	PeerSessionID string
	PeerName      string
	SharedFiles   []string
}

type SemanticOverlapInfo struct {
	PeerSessionID string
	PeerName      string
	ChangedFile   string
	DependentFile string
	Symbol        string
	Dependency    string
	Confidence    float64
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
	Name       string
	Status     session.Status
	Branch     string
	CWD        string
	RepoID     string
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
		Name:       value.Name,
		Status:     value.Status,
		Branch:     value.Branch,
		CWD:        value.CWD,
		RepoID:     value.RepoID,
		LastIntent: value.LastIntent,
		LastSeenAt: value.LastSeenAt,
		ParentID:   value.ParentID,
	}
}
