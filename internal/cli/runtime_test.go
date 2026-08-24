package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/juliancanalez/pane/internal/protocol"
)

type startCall struct {
	socket string
	log    string
}

// recordDaemonStarts substitutes the background-start seam so tests exercise the
// auto-start decision without spawning a real daemon.
func recordDaemonStarts(t *testing.T, result error) *[]startCall {
	t.Helper()
	calls := []startCall{}
	original := startDaemonBackground
	startDaemonBackground = func(socket, log string) error {
		calls = append(calls, startCall{socket: socket, log: log})
		return result
	}
	t.Cleanup(func() { startDaemonBackground = original })
	return &calls
}

type daemonPaths struct {
	socket string
	log    string
	stamp  string
}

func tempDaemonPaths(t *testing.T) daemonPaths {
	t.Helper()
	dir := t.TempDir()
	paths := daemonPaths{
		socket: filepath.Join(dir, "pane.sock"),
		log:    filepath.Join(dir, "pane.log"),
		stamp:  filepath.Join(dir, autostartStampName),
	}
	t.Setenv("PANE_SOCKET_PATH", paths.socket)
	t.Setenv("PANE_LOG_PATH", paths.log)
	t.Setenv("PANE_DB_PATH", filepath.Join(dir, "pane.db"))
	t.Setenv("PANE_PID_PATH", filepath.Join(dir, "pane.pid"))
	t.Setenv("PANE_NO_AUTOSTART", "")
	return paths
}

func TestSendDaemonRequestAutostartsDeadDaemonOnce(t *testing.T) {
	paths := tempDaemonPaths(t)
	calls := recordDaemonStarts(t, nil)

	_, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionStatus})
	if err == nil {
		t.Fatal("expected an error: nothing is listening on the socket after the retry")
	}
	if !strings.Contains(err.Error(), "Pane daemon is not running or is unreachable") {
		t.Fatalf("error message shape changed: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("start attempts = %d, want 1", len(*calls))
	}
	if (*calls)[0].socket != paths.socket || (*calls)[0].log != paths.log {
		t.Fatalf("start called with %+v, want socket %q log %q", (*calls)[0], paths.socket, paths.log)
	}
	if _, err := os.Stat(paths.stamp); err != nil {
		t.Fatalf("cooldown stamp not written: %v", err)
	}
}

func TestSendDaemonRequestSkipsAutostartWhenDisabled(t *testing.T) {
	paths := tempDaemonPaths(t)
	t.Setenv("PANE_NO_AUTOSTART", "1")
	calls := recordDaemonStarts(t, nil)

	if _, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionStatus}); err == nil {
		t.Fatal("expected an error")
	}
	if len(*calls) != 0 {
		t.Fatalf("start attempts = %d, want 0", len(*calls))
	}
	if _, err := os.Stat(paths.stamp); !os.IsNotExist(err) {
		t.Fatalf("stamp written despite PANE_NO_AUTOSTART: %v", err)
	}
}

func TestSendDaemonRequestSkipsAutostartForDaemonStop(t *testing.T) {
	paths := tempDaemonPaths(t)
	calls := recordDaemonStarts(t, nil)

	if _, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestDaemonStop}); err == nil {
		t.Fatal("expected an error")
	}
	if len(*calls) != 0 {
		t.Fatalf("stopping a dead daemon must not start one; attempts = %d", len(*calls))
	}
	if _, err := os.Stat(paths.stamp); !os.IsNotExist(err) {
		t.Fatalf("stamp written for a stop request: %v", err)
	}
}

func TestSendDaemonRequestAutostartCooldownRateLimitsAttempts(t *testing.T) {
	paths := tempDaemonPaths(t)
	calls := recordDaemonStarts(t, nil)

	for i := 0; i < 3; i++ {
		_, _ = sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionStatus})
	}
	if len(*calls) != 1 {
		t.Fatalf("start attempts inside the cooldown = %d, want 1", len(*calls))
	}

	expired := time.Now().Add(-2 * autostartCooldown)
	if err := os.Chtimes(paths.stamp, expired, expired); err != nil {
		t.Fatal(err)
	}
	_, _ = sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionStatus})
	if len(*calls) != 2 {
		t.Fatalf("start attempts after the cooldown expired = %d, want 2", len(*calls))
	}
}
