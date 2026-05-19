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
	fmt.Fprintf(&out, "Sessions: %d\n", len(value.Sessions))

	if len(value.Sessions) == 0 {
		fmt.Fprintf(&out, "\nNo active sessions found. Run `pane init` in a terminal pane to register one.\n")
		return out.String()
	}

	for _, item := range value.Sessions {
		fmt.Fprintf(&out, "\n%s — %s", item.ID, item.Status)
		if item.Branch != "" {
			fmt.Fprintf(&out, " — %s", item.Branch)
		}
		fmt.Fprintf(&out, "\n")
		fmt.Fprintf(&out, "  Intent: %s\n", displayIntent(item.LastIntent))
		fmt.Fprintf(&out, "  CWD: %s\n", displayPath(value.WorkspaceRoot, item.CWD))
		fmt.Fprintf(&out, "  Last seen: %s\n", relativeTime(item.LastSeenAt, now))
	}

	return out.String()
}

func displayIntent(intent string) string {
	if strings.TrimSpace(intent) == "" {
		return "not set"
	}
	return intent
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
