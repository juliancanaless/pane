package summary

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func Render(value StartupSummary, now time.Time) string {
	var out strings.Builder
	fmt.Fprintf(&out, "[Pane] Session summary\n")
	fmt.Fprintf(&out, "Workspace: %s\n", value.WorkspaceRoot)
	fmt.Fprintf(&out, "\nCurrent session: %s — %s", sessionLabel(value.Current), value.Current.Status)
	if value.Current.Branch != "" {
		fmt.Fprintf(&out, " — %s", value.Current.Branch)
	}
	fmt.Fprintf(&out, "\n")
	fmt.Fprintf(&out, "  Intent: %s\n", displayIntent(value.Current.LastIntent))
	fmt.Fprintf(&out, "  CWD: %s\n", displayPath(value.WorkspaceRoot, value.Current.CWD))
	fmt.Fprintf(&out, "  Last seen: %s\n", relativeTime(value.Current.LastSeenAt, now))
	if len(value.RecentFiles) > 0 {
		fmt.Fprintf(&out, "  Recent files: %s\n", strings.Join(displayPaths(value.WorkspaceRoot, value.RecentFiles), ", "))
	}

	renderCoordination(&out, value.Coordination, now)
	renderLineage(&out, value.Lineage, now)
	renderOverlaps(&out, value.Overlaps, value.WorkspaceRoot)
	renderSemanticOverlaps(&out, value.SemanticOverlaps, value.WorkspaceRoot)

	fmt.Fprintf(&out, "\nOther sessions: %d\n", len(value.Peers))
	if len(value.Peers) == 0 {
		fmt.Fprintf(&out, "  None currently visible in this workspace.\n")
		return out.String()
	}

	for _, peer := range value.Peers {
		fmt.Fprintf(&out, "\n%s — %s", sessionLabel(peer), peer.Status)
		if peer.Branch != "" {
			fmt.Fprintf(&out, " — %s", peer.Branch)
		}
		fmt.Fprintf(&out, "\n")
		fmt.Fprintf(&out, "  Intent: %s\n", displayIntent(peer.LastIntent))
		fmt.Fprintf(&out, "  CWD: %s\n", displayPath(value.WorkspaceRoot, peer.CWD))
		fmt.Fprintf(&out, "  Last seen: %s\n", relativeTime(peer.LastSeenAt, now))
	}

	return out.String()
}

func renderLineage(out *strings.Builder, lineage Lineage, now time.Time) {
	if lineage.Parent == nil && len(lineage.History) == 0 {
		return
	}
	fmt.Fprintf(out, "\nContinuity:\n")
	if lineage.Parent != nil {
		fmt.Fprintf(out, "  Continued from: %s (%s) — %s\n", lineage.Parent.SessionID, relativeTime(lineage.Parent.LastSeenAt, now), displayIntent(lineage.Parent.LastIntent))
	}
	if len(lineage.History) > 0 {
		fmt.Fprintf(out, "  Recent workspace history:\n")
		for _, item := range lineage.History {
			fmt.Fprintf(out, "  - %s (%s", item.SessionID, relativeTime(item.LastSeenAt, now))
			if item.Branch != "" {
				fmt.Fprintf(out, ", %s", item.Branch)
			}
			fmt.Fprintf(out, "): %s\n", displayIntent(item.LastIntent))
		}
	}
}

func renderOverlaps(out *strings.Builder, overlaps []OverlapInfo, workspaceRoot string) {
	if len(overlaps) == 0 {
		return
	}
	fmt.Fprintf(out, "\nOverlap:\n")
	for _, overlap := range overlaps {
		label := overlap.PeerSessionID
		if overlap.PeerName != "" {
			label = overlap.PeerName
		}
		fmt.Fprintf(out, "  ⚠️  %s shares: %s\n", label, strings.Join(displayPaths(workspaceRoot, overlap.SharedFiles), ", "))
	}
}

func renderSemanticOverlaps(out *strings.Builder, overlaps []SemanticOverlapInfo, workspaceRoot string) {
	if len(overlaps) == 0 {
		return
	}
	fmt.Fprintf(out, "\nSemantic overlap:\n")
	for _, overlap := range overlaps {
		label := overlap.PeerSessionID
		if overlap.PeerName != "" {
			label = overlap.PeerName
		}
		symbol := overlap.Symbol
		if symbol == "" {
			symbol = overlap.Dependency
		}
		fmt.Fprintf(out, "  ⚠️  %s depends on %s changed in %s via %s in %s\n", label, symbol, displayPath(workspaceRoot, overlap.ChangedFile), overlap.Dependency, displayPath(workspaceRoot, overlap.DependentFile))
	}
}

func renderCoordination(out *strings.Builder, coordination Coordination, now time.Time) {
	if len(coordination.UnreadMessages) == 0 && coordination.AwaitingReplies == 0 {
		return
	}
	fmt.Fprintf(out, "\nCoordination:\n")
	if len(coordination.UnreadMessages) > 0 {
		fmt.Fprintf(out, "  Unread messages: %d\n", len(coordination.UnreadMessages))
		for _, message := range coordination.UnreadMessages {
			fmt.Fprintf(out, "  - %s from %s (%s): %s\n", message.ID, message.FromSession, relativeTime(message.CreatedAt, now), message.Body)
		}
	}
	if coordination.AwaitingReplies > 0 {
		fmt.Fprintf(out, "  Awaiting replies: %d\n", coordination.AwaitingReplies)
	}
}

func sessionLabel(line SessionLine) string {
	if line.Name != "" {
		return line.SessionID + " (" + line.Name + ")"
	}
	return line.SessionID
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
