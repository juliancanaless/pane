package session

import "testing"

func clearMultiplexerEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"PANE_PANE_ID", "CMUX_SURFACE_ID", "ZELLIJ_PANE_ID", "ZELLIJ_SESSION_NAME", "TMUX_PANE"} {
		t.Setenv(key, "")
	}
}

func TestDetectPaneIDPrefersPaneOverride(t *testing.T) {
	clearMultiplexerEnv(t)
	t.Setenv("PANE_PANE_ID", "spawn:abc")
	t.Setenv("CMUX_SURFACE_ID", "3D10F6CE")
	t.Setenv("ZELLIJ_PANE_ID", "12")
	t.Setenv("TMUX_PANE", "%1")

	got := DetectPaneID("/dev/ttys001")
	if got != "spawn:abc" {
		t.Fatalf("pane id = %q", got)
	}
}

func TestDetectPaneIDFallsBackToTTY(t *testing.T) {
	clearMultiplexerEnv(t)

	got := DetectPaneID("/dev/ttys001")
	if got != "tty:/dev/ttys001" {
		t.Fatalf("pane id = %q", got)
	}
}

func TestDetectPaneIDUsesCmuxWithoutInnerMultiplexer(t *testing.T) {
	clearMultiplexerEnv(t)
	t.Setenv("CMUX_SURFACE_ID", "3D10F6CE")

	got := DetectPaneID("/dev/ttys001")
	if got != "cmux:3D10F6CE" {
		t.Fatalf("pane id = %q", got)
	}
}

func TestDetectPaneIDPrefersInnerMultiplexerOverCmux(t *testing.T) {
	// zellij panes inside one cmux surface all share CMUX_SURFACE_ID; the
	// inner multiplexer is the real pane identity.
	clearMultiplexerEnv(t)
	t.Setenv("CMUX_SURFACE_ID", "3D10F6CE")
	t.Setenv("ZELLIJ_PANE_ID", "12")
	t.Setenv("ZELLIJ_SESSION_NAME", "control-center")

	if got := DetectPaneID("/dev/ttys001"); got != "zellij:control-center:12" {
		t.Fatalf("pane id = %q", got)
	}

	t.Setenv("ZELLIJ_PANE_ID", "")
	t.Setenv("ZELLIJ_SESSION_NAME", "")
	t.Setenv("TMUX_PANE", "%1")
	if got := DetectPaneID("/dev/ttys001"); got != "tmux:%1" {
		t.Fatalf("pane id = %q", got)
	}
}

func TestDetectPaneIDPrefersZellij(t *testing.T) {
	clearMultiplexerEnv(t)
	t.Setenv("ZELLIJ_PANE_ID", "12")
	t.Setenv("TMUX_PANE", "%1")

	got := DetectPaneID("/dev/ttys001")
	if got != "zellij:12" {
		t.Fatalf("pane id = %q", got)
	}
}

func TestDetectPaneIDIncludesZellijSessionName(t *testing.T) {
	clearMultiplexerEnv(t)
	t.Setenv("ZELLIJ_PANE_ID", "12")
	t.Setenv("ZELLIJ_SESSION_NAME", "main")

	got := DetectPaneID("/dev/ttys001")
	if got != "zellij:main:12" {
		t.Fatalf("pane id = %q", got)
	}
}
