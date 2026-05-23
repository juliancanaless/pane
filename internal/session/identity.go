package session

import "os"

func DetectPaneID(tty string) string {
	if value := os.Getenv("PANE_PANE_ID"); value != "" {
		return value
	}
	if value := os.Getenv("ZELLIJ_PANE_ID"); value != "" {
		return "zellij:" + value
	}
	if value := os.Getenv("TMUX_PANE"); value != "" {
		return "tmux:" + value
	}
	if tty != "" {
		return "tty:" + tty
	}
	return "unknown"
}
