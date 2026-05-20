package messages

import (
	"fmt"
	"strings"
	"time"
)

func RenderInbox(items []Message, now time.Time) string {
	var out strings.Builder
	fmt.Fprintf(&out, "[Pane] Inbox\n")
	fmt.Fprintf(&out, "Messages: %d\n", len(items))
	if len(items) == 0 {
		fmt.Fprintf(&out, "\nNo unread coordination messages.\n")
		return out.String()
	}
	for _, item := range items {
		fmt.Fprintf(&out, "\n%s — from %s — %s\n", item.ID, item.FromSession, relativeTime(item.CreatedAt, now))
		fmt.Fprintf(&out, "  Thread: %s\n", item.ThreadID)
		fmt.Fprintf(&out, "  %s\n", item.Body)
	}
	return out.String()
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
