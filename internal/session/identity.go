package session

import "os"

func DetectPaneID(tty string) string {
	if value := os.Getenv("PANE_PANE_ID"); value != "" {
		return value
	}
	// The innermost multiplexer wins: every zellij/tmux pane inside a cmux
	// surface inherits that surface's CMUX_SURFACE_ID, so cmux identifies the
	// pane only when no inner multiplexer is present.
	if value := os.Getenv("ZELLIJ_PANE_ID"); value != "" {
		// The session name makes the pane addressable from outside (the
		// daemon's wake nudge runs detached from any zellij session).
		if sessionName := os.Getenv("ZELLIJ_SESSION_NAME"); sessionName != "" {
			return "zellij:" + sessionName + ":" + value
		}
		return "zellij:" + value
	}
	if value := os.Getenv("TMUX_PANE"); value != "" {
		return "tmux:" + value
	}
	if value := os.Getenv("CMUX_SURFACE_ID"); value != "" {
		return "cmux:" + value
	}
	if tty != "" {
		return "tty:" + tty
	}
	return "unknown"
}
