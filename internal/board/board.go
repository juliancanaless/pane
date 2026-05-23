package board

import "github.com/juliancanalez/pane/internal/session"

type Board struct {
	WorkspaceRoot    string
	RepoID           string
	Scope            string
	Sessions         []Session
	Overlaps         []OverlapInfo
	SemanticOverlaps []SemanticOverlapInfo
	RecentGitEvents  []GitEventInfo
}

type GitEventInfo struct {
	SessionShortID string
	SessionName    string
	Command        string
	Timestamp      int64
}

type OverlapInfo struct {
	SessionA    string
	SessionB    string
	SharedFiles []string
}

type SemanticOverlapInfo struct {
	SourceSession    string
	DependentSession string
	ChangedFile      string
	DependentFile    string
	Symbol           string
	Dependency       string
	Confidence       float64
}

type Session struct {
	ID                string
	ShortID           string
	Name              string
	Status            session.Status
	Branch            string
	CWD               string
	WorkspaceRoot     string
	RepoID            string
	ParentID          string
	LastIntent        string
	LastSeenAt        int64
	UnreadMessages    int
	AwaitingReplies   int
	RecentFiles       []string
	HotDirectories    []string
	ActivitySummaries []string
}

func FromSessions(workspaceRoot string, sessions []session.Session) Board {
	return FromSessionsWithMessages(workspaceRoot, sessions, nil)
}

func FromSessionsWithMessages(workspaceRoot string, sessions []session.Session, stats map[string]MessageStats) Board {
	return FromSessionsWithStats(workspaceRoot, sessions, stats, nil)
}

func FromSessionsWithStats(workspaceRoot string, sessions []session.Session, messageStats map[string]MessageStats, activityStats map[string]ActivityStats) Board {
	items := make([]Session, 0, len(sessions))
	for _, value := range sessions {
		messages := messageStats[value.ID]
		activity := activityStats[value.ID]
		items = append(items, Session{
			ID:                value.ID,
			ShortID:           session.ShortID(value.ID),
			Name:              value.Name,
			Status:            value.Status,
			Branch:            value.Branch,
			CWD:               value.CWD,
			WorkspaceRoot:     value.WorkspaceRoot,
			RepoID:            value.RepoID,
			ParentID:          value.ParentID,
			LastIntent:        value.LastIntent,
			LastSeenAt:        value.LastSeenAt,
			UnreadMessages:    messages.UnreadMessages,
			AwaitingReplies:   messages.AwaitingReplies,
			RecentFiles:       activity.RecentFiles,
			HotDirectories:    activity.HotDirectories,
			ActivitySummaries: activity.ActivitySummaries,
		})
	}
	return Board{WorkspaceRoot: workspaceRoot, Sessions: items}
}

type MessageStats struct {
	UnreadMessages  int
	AwaitingReplies int
}

type ActivityStats struct {
	RecentFiles       []string
	HotDirectories    []string
	ActivitySummaries []string
}
