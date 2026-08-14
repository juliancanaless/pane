package daemon

import (
	"strings"
	"testing"
)

func TestWakeCommandTargetsMultiplexers(t *testing.T) {
	t.Setenv("CMUX_BUNDLED_CLI_PATH", "")

	name, args := wakeCommand("cmux:3D10F6CE")
	if name != "cmux" || strings.Join(args, " ") != `send --surface 3D10F6CE -- pane inbox\n` {
		t.Fatalf("cmux wake = %q %v", name, args)
	}

	name, args = wakeCommand("zellij:main:12")
	if name != "zellij" || strings.Join(args, " ") != "--session main action write-chars --pane-id 12 pane inbox\r" {
		t.Fatalf("zellij wake = %q %v", name, args)
	}

	name, args = wakeCommand("tmux:%1")
	if name != "tmux" || strings.Join(args, " ") != "send-keys -t %1 pane inbox Enter" {
		t.Fatalf("tmux wake = %q %v", name, args)
	}
}

func TestWakeCommandSkipsUnaddressablePanes(t *testing.T) {
	for _, paneID := range []string{"tty:/dev/ttys001", "zellij:12", "unknown", "spawn:abc"} {
		if name, _ := wakeCommand(paneID); name != "" {
			t.Fatalf("pane %q must not be woken, got command %q", paneID, name)
		}
	}
}

func TestCmuxBinaryPrefersBundledPath(t *testing.T) {
	t.Setenv("CMUX_BUNDLED_CLI_PATH", "/Applications/cmux.app/Contents/Resources/bin/cmux")
	if got := cmuxBinary(); got != "/Applications/cmux.app/Contents/Resources/bin/cmux" {
		t.Fatalf("cmux binary = %q", got)
	}
	t.Setenv("CMUX_BUNDLED_CLI_PATH", "")
	if got := cmuxBinary(); got != "cmux" {
		t.Fatalf("cmux binary fallback = %q", got)
	}
}
