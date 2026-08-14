package daemon

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/juliancanalez/pane/internal/session"
)

// wakeDebounce suppresses repeat keystroke injections into the same terminal
// pane while the first nudge is presumably still being handled.
const wakeDebounce = 30 * time.Second

var wakeMu sync.Mutex
var lastWake = map[string]time.Time{}

// wakeTarget nudges the terminal pane of a message recipient so an idle agent
// notices the new message instead of waiting for its next turn. Keystroke
// injection is only possible under a terminal multiplexer, and only sessions
// bound to an agent session (hooks installed) are nudged: for those, typing
// "pane inbox" either lands in the agent's prompt as an instruction it knows
// how to follow, or in a plain shell as a harmless command.
func wakeTarget(target session.Session) {
	if os.Getenv("PANE_WAKE") == "off" || target.AgentSessionID == "" {
		return
	}
	name, args := wakeCommand(target.PaneID)
	if name == "" {
		return
	}
	wakeMu.Lock()
	if last, seen := lastWake[target.PaneID]; seen && time.Since(last) < wakeDebounce {
		wakeMu.Unlock()
		return
	}
	lastWake[target.PaneID] = time.Now()
	wakeMu.Unlock()
	_ = exec.Command(name, args...).Run()
}

// wakeCommand builds the multiplexer-specific keystroke injection for a pane
// id. An empty name means the pane is not addressable from outside (plain
// tty, or a zellij id recorded without its session name).
func wakeCommand(paneID string) (string, []string) {
	if ref, ok := strings.CutPrefix(paneID, "cmux:"); ok {
		// cmux interprets a literal \n escape sequence as Enter.
		return cmuxBinary(), []string{"send", "--surface", ref, "--", `pane inbox\n`}
	}
	if ref, ok := strings.CutPrefix(paneID, "zellij:"); ok {
		sessionName, paneRef, found := strings.Cut(ref, ":")
		if !found {
			return "", nil
		}
		return "zellij", []string{"--session", sessionName, "action", "write-chars", "--pane-id", paneRef, "pane inbox\r"}
	}
	if ref, ok := strings.CutPrefix(paneID, "tmux:"); ok {
		return "tmux", []string{"send-keys", "-t", ref, "pane inbox", "Enter"}
	}
	return "", nil
}

// cmuxBinary resolves the cmux CLI: the surface's bundled CLI path when the
// daemon inherited a cmux environment, else whatever is on PATH.
func cmuxBinary() string {
	if path := os.Getenv("CMUX_BUNDLED_CLI_PATH"); path != "" {
		return path
	}
	return "cmux"
}
