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
func StartBackground(socketPath string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	// Build args: we call ourselves with "daemon start" (without --background)
	args := []string{executable, "daemon", "start"}

	devNull, err := os.Open(os.DevNull)
	if err != nil {
		return err
	}
	defer devNull.Close()

	attr := &os.ProcAttr{
		Dir:   "/",
		Files: []*os.File{devNull, devNull, devNull}, // stdin, stdout, stderr → /dev/null
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
func Restart(socketPath string) error {
	client := Client{SocketPath: socketPath, Timeout: 1 * time.Second}
	if _, err := client.Send(protocol.Request{Type: protocol.RequestDaemonStop}); err == nil {
		for i := 0; i < 30; i++ {
			time.Sleep(100 * time.Millisecond)
			if _, err := client.Send(protocol.Request{Type: protocol.RequestDaemonHealth}); err != nil {
				break
			}
		}
	}
	return StartBackground(socketPath)
}
