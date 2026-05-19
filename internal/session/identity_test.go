package session

import "testing"

func TestDetectPaneIDFallsBackToTTY(t *testing.T) {
	t.Setenv("ZELLIJ_PANE_ID", "")
	t.Setenv("TMUX_PANE", "")

	got := DetectPaneID("/dev/ttys001")
	if got != "tty:/dev/ttys001" {
		t.Fatalf("pane id = %q", got)
	}
}

func TestDetectPaneIDPrefersZellij(t *testing.T) {
	t.Setenv("ZELLIJ_PANE_ID", "12")
	t.Setenv("TMUX_PANE", "%1")

	got := DetectPaneID("/dev/ttys001")
	if got != "zellij:12" {
		t.Fatalf("pane id = %q", got)
	}
}
