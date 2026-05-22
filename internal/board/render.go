package board

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func Render(value Board, now time.Time) string {
	var out strings.Builder
	fmt.Fprintf(&out, "[Pane] Workspace board\n")
	fmt.Fprintf(&out, "Workspace: %s\n", value.WorkspaceRoot)
	if value.Scope == "repo" && value.RepoID != "" {
		fmt.Fprintf(&out, "Scope: repository (%s)\n", value.RepoID)
	}
	fmt.Fprintf(&out, "Sessions: %d\n", len(value.Sessions))

	if len(value.Sessions) == 0 {
		fmt.Fprintf(&out, "\nNo active sessions found. Run `pane init` in a terminal pane to register one.\n")
		return out.String()
	}

	for _, item := range value.Sessions {
		fmt.Fprintf(&out, "\n%s", item.ID)
		if item.Name != "" {
			fmt.Fprintf(&out, " (%s)", item.Name)
		} else if item.ShortID != "" && item.ShortID != item.ID {
			fmt.Fprintf(&out, " (short: %s)", item.ShortID)
		}
		fmt.Fprintf(&out, " — %s", item.Status)
		if item.Branch != "" {
			fmt.Fprintf(&out, " — %s", item.Branch)
		}
		fmt.Fprintf(&out, "\n")
		fmt.Fprintf(&out, "  Intent: %s\n", displayIntent(item.LastIntent))
		workspaceRoot := value.WorkspaceRoot
		if item.WorkspaceRoot != "" {
			workspaceRoot = item.WorkspaceRoot
		}
		fmt.Fprintf(&out, "  CWD: %s\n", displayPath(workspaceRoot, item.CWD))
		if value.Scope == "repo" && item.WorkspaceRoot != "" && item.WorkspaceRoot != value.WorkspaceRoot {
			fmt.Fprintf(&out, "  Worktree: %s\n", item.WorkspaceRoot)
		}
		fmt.Fprintf(&out, "  Last seen: %s\n", relativeTime(item.LastSeenAt, now))
		if len(item.RecentFiles) > 0 {
			fmt.Fprintf(&out, "  Recent files: %s\n", strings.Join(displayPaths(workspaceRoot, item.RecentFiles), ", "))
		}
		if len(item.HotDirectories) > 0 {
			fmt.Fprintf(&out, "  Hot dirs: %s\n", strings.Join(displayPaths(workspaceRoot, item.HotDirectories), ", "))
		}
		if len(item.ActivitySummaries) > 0 {
			fmt.Fprintf(&out, "  Activity summary: %s\n", strings.Join(item.ActivitySummaries, "; "))
		}
		if item.UnreadMessages > 0 || item.AwaitingReplies > 0 {
			fmt.Fprintf(&out, "  Coordination: %s\n", coordinationSummary(item.UnreadMessages, item.AwaitingReplies))
		}
	}

	if len(value.Overlaps) > 0 {
		fmt.Fprintf(&out, "\nOverlap:\n")
		for _, overlap := range value.Overlaps {
			shortA := overlap.SessionA
			shortB := overlap.SessionB
			// Prefer name, then short ID
			for _, s := range value.Sessions {
				if s.ID == overlap.SessionA {
					if s.Name != "" {
						shortA = s.Name
					} else if s.ShortID != "" {
						shortA = s.ShortID
					}
				}
				if s.ID == overlap.SessionB {
					if s.Name != "" {
						shortB = s.Name
					} else if s.ShortID != "" {
						shortB = s.ShortID
					}
				}
			}
			fmt.Fprintf(&out, "  ⚠️  %s ↔ %s: %s\n", shortA, shortB, strings.Join(displayPaths(value.WorkspaceRoot, overlap.SharedFiles), ", "))
		}
	}

	if len(value.SemanticOverlaps) > 0 {
		fmt.Fprintf(&out, "\nSemantic overlap:\n")
		for _, overlap := range value.SemanticOverlaps {
			source := sessionDisplayName(value.Sessions, overlap.SourceSession)
			dependent := sessionDisplayName(value.Sessions, overlap.DependentSession)
			symbol := overlap.Symbol
			if symbol == "" {
				symbol = overlap.Dependency
			}
			fmt.Fprintf(&out, "  ⚠️  %s changed %s in %s; %s depends via %s in %s\n", source, symbol, displayPath(value.WorkspaceRoot, overlap.ChangedFile), dependent, overlap.Dependency, displayPath(value.WorkspaceRoot, overlap.DependentFile))
		}
	}

	if len(value.RecentGitEvents) > 0 {
		fmt.Fprintf(&out, "\nRecent git:\n")
		for _, event := range value.RecentGitEvents {
			label := event.SessionShortID
			if event.SessionName != "" {
				label = event.SessionName
			}
			fmt.Fprintf(&out, "  %s — %s (%s)\n", label, event.Command, relativeTime(event.Timestamp, now))
		}
	}

	return out.String()
}

func sessionDisplayName(sessions []Session, id string) string {
	for _, item := range sessions {
		if item.ID != id {
			continue
		}
		if item.Name != "" {
			return item.Name
		}
		if item.ShortID != "" {
			return item.ShortID
		}
	}
	return id
}

func coordinationSummary(unread, awaiting int) string {
	parts := make([]string, 0, 2)
	if unread > 0 {
		parts = append(parts, fmt.Sprintf("%d unread", unread))
	}
	if awaiting > 0 {
		parts = append(parts, fmt.Sprintf("%d awaiting reply", awaiting))
	}
	return strings.Join(parts, ", ")
}

func displayIntent(intent string) string {
	if strings.TrimSpace(intent) == "" {
		return "not set"
	}
	return intent
}

func displayPaths(workspaceRoot string, paths []string) []string {
	values := make([]string, 0, len(paths))
	for _, path := range paths {
		values = append(values, displayPath(workspaceRoot, path))
	}
	return values
}

func displayPath(workspaceRoot, path string) string {
	if path == "" {
		return "unknown"
	}
	if workspaceRoot == "" {
		return path
	}
	relative, err := filepath.Rel(workspaceRoot, path)
	if err != nil || strings.HasPrefix(relative, "..") {
		return path
	}
	if relative == "." {
		return "."
	}
	return relative
}

func relativeTime(timestamp int64, now time.Time) string {
	if timestamp <= 0 {
		return "unknown"
	}
	delta := now.Sub(time.Unix(timestamp, 0))
	if delta < 0 {
		delta = 0
	}
	if delta < time.Minute {
		return fmt.Sprintf("%ds ago", int(delta.Seconds()))
	}
	if delta < time.Hour {
		return fmt.Sprintf("%dm ago", int(delta.Minutes()))
	}
	if delta < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(delta.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(delta.Hours()/24))
}
