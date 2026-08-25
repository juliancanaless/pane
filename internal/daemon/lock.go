package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// AcquireLock tries to get an exclusive flock on the lock file.
// Returns the file (caller must keep it open for the lock to hold) or an error.
func AcquireLock(lockPath string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("another daemon is already running (lock: %s)", lockPath)
	}
	// Write our PID into the lock file for diagnostics
	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	_ = f.Sync()
	return f, nil
}

// ReleaseLock unlocks and closes the lock file, leaving the file in place so
// every acquirer contends on the same inode. Removing it opens a race: a
// starter that opened the file before the unlink keeps a lock on the old
// inode while the next starter creates and locks a fresh one, and both
// daemons believe they hold the lock.
func ReleaseLock(f *os.File) {
	if f == nil {
		return
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}

// CleanStale checks whether a previous daemon is still alive.
// If the PID file exists and the process is dead, it cleans up stale
// socket and PID files. Returns true if stale state was cleaned.
// Returns an error if the old process is still alive.
func CleanStale(pidPath, socketPath string) (cleaned bool, err error) {
	pid, err := readPIDFile(pidPath)
	if err != nil {
		// No PID file or unreadable — check socket existence
		if _, statErr := os.Stat(socketPath); statErr == nil {
			// Stale socket with no PID file — safe to remove
			_ = os.Remove(socketPath)
			return true, nil
		}
		return false, nil
	}

	// Only treat the PID as a live daemon if it's both alive AND actually a
	// pane process. After a reboot the OS can recycle the old daemon's PID to
	// an unrelated process (e.g. a system agent); a bare liveness check would
	// then falsely report "daemon already running" forever, since the dead
	// daemon's socket no longer accepts connections. Verifying identity makes
	// recycled PIDs read as stale so we clean up and restart cleanly.
	if processAlive(pid) && processIsPaneDaemon(pid) {
		return false, fmt.Errorf("daemon already running (pid %d from %s)", pid, pidPath)
	}

	// Process is dead, or the PID was recycled to a non-pane process — clean
	// up stale files.
	_ = os.Remove(pidPath)
	_ = os.Remove(socketPath)
	return true, nil
}

// processIsPaneDaemon reports whether the given live PID is actually a pane
// process, guarding against PID recycling. It inspects the process's command
// name via ps (portable across macOS and Linux). On any uncertainty it returns
// false, so callers treat the state as stale rather than blocking a restart.
// It is a var so tests can substitute a deterministic check.
var processIsPaneDaemon = func(pid int) bool {
	if pid <= 0 {
		return false
	}
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return false
	}
	name := strings.TrimSpace(string(out))
	if name == "" {
		return false
	}
	return strings.Contains(strings.ToLower(filepath.Base(name)), "pane")
}

func readPIDFile(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(data)))
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
