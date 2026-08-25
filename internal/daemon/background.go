package daemon

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/juliancanalez/pane/internal/protocol"
)

// StartBackground starts the daemon detached and waits up to 3 seconds for it
// to become healthy. When the launchd agent is registered (macOS), launchd is
// asked to start its job instead of spawning directly — a directly spawned
// daemon would hold the flock and leave launchd's KeepAlive child in a
// respawn loop against it.
func StartBackground(socketPath, logPath string) error {
	if launchAgentLoaded() {
		if err := kickstartLaunchAgent(); err == nil {
			return waitUntilHealthy(socketPath)
		}
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	// --foreground, or the child would background itself again and fork forever.
	args := []string{executable, "daemon", "start", "--foreground"}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	defer devNull.Close()

	// The child's stderr goes to the log so a panic before the daemon sets up
	// its own logging leaves a trace instead of vanishing into /dev/null.
	childStderr := devNull
	if logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		defer logFile.Close()
		childStderr = logFile
	}

	attr := &os.ProcAttr{
		Dir:   "/",
		Files: []*os.File{devNull, devNull, childStderr},
		Sys: &syscall.SysProcAttr{
			Setsid: true, // new session — fully detached
		},
	}

	proc, err := os.StartProcess(executable, args, attr)
	if err != nil {
		return fmt.Errorf("start background daemon: %w", err)
	}
	// Release the process so it doesn't become a zombie
	_ = proc.Release()

	return waitUntilHealthy(socketPath)
}

func waitUntilHealthy(socketPath string) error {
	client := Client{SocketPath: socketPath, Timeout: 1 * time.Second}
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		resp, err := client.Send(protocol.Request{Type: protocol.RequestDaemonHealth})
		if err == nil && resp.OK {
			return nil
		}
	}
	return fmt.Errorf("daemon started but did not become healthy within 3 seconds")
}

// Restart asks the daemon on socketPath to stop, waits for it to release the
// socket, and starts a fresh background daemon from the current executable.
// Used to replace a daemon left running by an older install.
func Restart(socketPath, logPath string) error {
	client := Client{SocketPath: socketPath, Timeout: 1 * time.Second}
	if _, err := client.Send(protocol.Request{Type: protocol.RequestDaemonStop}); err == nil {
		for i := 0; i < 30; i++ {
			time.Sleep(100 * time.Millisecond)
			if _, err := client.Send(protocol.Request{Type: protocol.RequestDaemonHealth}); err != nil {
				break
			}
		}
	}
	return StartBackground(socketPath, logPath)
}
