package summary

type StartupSummary struct {
	SessionID      string
	Branch         string
	ActiveSessions []SessionLine
	Overlap        []string
	Coordination   []string
}

type SessionLine struct {
	SessionID   string
	Branch      string
	RecentFiles []string
}
