package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/juliancanalez/pane/internal/protocol"
)

// launchdLabel identifies the pane daemon job in the user's launchd domain.
const launchdLabel = "com.pane.daemon"

// runLaunchctl is a var so tests can stub launchctl.
var runLaunchctl = func(args ...string) error {
	out, err := exec.Command("launchctl", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("launchctl %v: %v: %s", args, err, out)
	}
	return nil
}

func launchdService() string {
	return fmt.Sprintf("gui/%d/%s", os.Getuid(), launchdLabel)
}

// LaunchAgentPlistPath returns where the pane LaunchAgent plist lives.
func LaunchAgentPlistPath(home string) string {
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
}

// LaunchAgentPlist renders the LaunchAgent definition. KeepAlive on
// SuccessfulExit=false means launchd respawns the daemon after a crash or
// kill but leaves it down after a clean exit, so `pane daemon stop` sticks.
// StandardErrorPath catches output from before the daemon redirects its own
// stderr to the log (flag errors, lock contention).
func LaunchAgentPlist(execPath, logPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>daemon</string>
		<string>start</string>
		<string>--foreground</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>ProcessType</key>
	<string>Background</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, launchdLabel, execPath, logPath)
}

// launchAgentLoaded reports whether the pane LaunchAgent is registered in the
// user's launchd domain. Always false off macOS. A var so tests can stub it.
var launchAgentLoaded = func() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	return runLaunchctl("print", launchdService()) == nil
}

// LaunchAgentLoaded reports whether the pane LaunchAgent is registered in the
// user's launchd domain.
func LaunchAgentLoaded() bool { return launchAgentLoaded() }

// kickstartLaunchAgent asks launchd to start the registered daemon job now if
// it is not already running. A var so tests can stub it.
var kickstartLaunchAgent = func() error {
	return runLaunchctl("kickstart", launchdService())
}

// InstallLaunchAgent registers the daemon as a launchd LaunchAgent so macOS
// respawns it after a crash. Any already-running daemon is stopped first:
// launchd's child would otherwise lose the flock to it and respawn-loop.
// RunAtLoad starts the daemon as part of bootstrap.
func InstallLaunchAgent(execPath, socketPath, logPath, home string) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("launchd is only available on macOS")
	}
	plistPath := LaunchAgentPlistPath(home)
	desired := LaunchAgentPlist(execPath, logPath)
	if existing, err := os.ReadFile(plistPath); err == nil && string(existing) == desired && launchAgentLoaded() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(plistPath, []byte(desired), 0o644); err != nil {
		return err
	}
	// Drop any previous registration so a changed plist takes effect. This
	// also SIGTERMs a launchd-owned daemon; errors just mean nothing was
	// registered.
	_ = runLaunchctl("bootout", launchdService())
	stopRunningDaemon(socketPath)
	return runLaunchctl("bootstrap", fmt.Sprintf("gui/%d", os.Getuid()), plistPath)
}

// stopRunningDaemon cleanly stops a daemon reachable on socketPath and waits
// for it to release the socket. No-op when none is running.
func stopRunningDaemon(socketPath string) {
	client := Client{SocketPath: socketPath, Timeout: 1 * time.Second}
	if _, err := client.Send(protocol.Request{Type: protocol.RequestDaemonStop}); err != nil {
		return
	}
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if _, err := client.Send(protocol.Request{Type: protocol.RequestDaemonHealth}); err != nil {
			return
		}
	}
}
