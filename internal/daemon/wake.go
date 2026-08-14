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
	paneRef, ok := strings.CutPrefix(target.PaneID, "tmux:")
	if !ok {
		return
	}
	wakeMu.Lock()
	if last, seen := lastWake[target.PaneID]; seen && time.Since(last) < wakeDebounce {
		wakeMu.Unlock()
		return
	}
	lastWake[target.PaneID] = time.Now()
	wakeMu.Unlock()
	_ = exec.Command("tmux", "send-keys", "-t", paneRef, "pane inbox", "Enter").Run()
}
