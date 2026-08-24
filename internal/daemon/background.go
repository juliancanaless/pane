package daemon

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/juliancanalez/pane/internal/protocol"
)

// StartBackground re-execs the daemon binary as a detached child process.
// The parent waits up to 3 seconds for the child to become healthy,
// then exits. Returns an error if the child fails to start.
func StartBackground(socketPath, logPath string) error {
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

	// Wait for the daemon to become healthy
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
