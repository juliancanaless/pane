package cli

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/juliancanalez/pane/internal/daemon"
	"github.com/juliancanalez/pane/internal/protocol"
	"github.com/juliancanalez/pane/internal/store"
	"github.com/juliancanalez/pane/internal/version"
)

const (
	autostartStampName = "autostart-stamp"
	autostartCooldown  = 10 * time.Second
)

// startDaemonBackground is a var so tests can substitute a recorder instead of
// spawning a real daemon.
var startDaemonBackground = daemon.StartBackground

func sendDaemonRequest(request protocol.Request) (protocol.Response, error) {
	socket, err := socketPath()
	if err != nil {
		return protocol.Response{}, err
	}
	client := daemon.Client{SocketPath: socket}
	response, err := client.Send(request)
	if err != nil {
		// Agent tool calls kill their process group, which takes any daemon
		// started inside one with them. Bringing it back here keeps every
		// command working without the agent noticing the daemon was gone.
		if !isConnectionError(err) || !autostartDaemon(socket, request.Type) {
			return protocol.Response{}, daemonUnreachable(err)
		}
		if response, err = client.Send(request); err != nil {
			return protocol.Response{}, daemonUnreachable(err)
		}
	}
	maybeRestartStaleDaemon(socket, request.Type, response.DaemonVersion)
	return response, nil
}

func daemonUnreachable(err error) error {
	return fmt.Errorf("Pane daemon is not running or is unreachable; start it with `pane daemon start`: %w", err)
}

// isConnectionError reports whether nothing was listening on the socket. A
// failure after the connection is up means a daemon is there but sick, and
// starting another one would only trip its lock.
func isConnectionError(err error) bool {
	var opErr *net.OpError
	return errors.As(err, &opErr) && opErr.Op == "dial"
}

// autostartDaemon spawns a detached daemon and reports whether the request is
// worth retrying. The cooldown stamp is written before the attempt so a daemon
// that cannot start is retried every 10 seconds rather than on every command.
// The whole path stays silent: hooks and the statusline run through it.
func autostartDaemon(socket string, requestType protocol.RequestType) bool {
	if requestType == protocol.RequestDaemonStop || os.Getenv("PANE_NO_AUTOSTART") != "" {
		return false
	}
	stamp := filepath.Join(filepath.Dir(socket), autostartStampName)
	if info, err := os.Stat(stamp); err == nil && now().Sub(info.ModTime()) < autostartCooldown {
		return false
	}
	if err := os.MkdirAll(filepath.Dir(stamp), 0o755); err != nil {
		return false
	}
	if err := os.WriteFile(stamp, nil, 0o644); err != nil {
		return false
	}
	log, _ := logPath()
	return startDaemonBackground(socket, log) == nil
}

// maybeRestartStaleDaemon replaces a daemon running an older release than this
// CLI (an empty version means a pre-0.1.5 daemon). The triggering request was
// already served by the old daemon, so nothing is retried; the next command
// simply reaches the new daemon. Never downgrades a newer daemon.
func maybeRestartStaleDaemon(socket string, requestType protocol.RequestType, daemonVersion string) {
	if requestType == protocol.RequestDaemonStop || !version.IsOlder(daemonVersion, version.Version) {
		return
	}
	if daemonVersion == "" {
		daemonVersion = "pre-0.1.5"
	}
	fmt.Fprintf(os.Stderr, "[Pane] daemon (%s) is older than this CLI (%s); restarting daemon with the current binary\n", daemonVersion, version.Version)
	log, _ := logPath()
	if err := daemon.Restart(socket, log); err != nil {
		fmt.Fprintf(os.Stderr, "[Pane] daemon restart failed: %v\n", err)
	}
}

func logPath() (string, error) {
	if value := os.Getenv("PANE_LOG_PATH"); value != "" {
		return value, nil
	}
	if value := os.Getenv("PANE_LOG"); value != "" {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return store.DefaultLogPath(home), nil
}

func pidPath() (string, error) {
	if value := os.Getenv("PANE_PID_PATH"); value != "" {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return store.DefaultPIDPath(home), nil
}

func databasePath() (string, error) {
	if value := os.Getenv("PANE_DB_PATH"); value != "" {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return store.DefaultDBPath(home), nil
}

var now = time.Now

func socketPath() (string, error) {
	if value := os.Getenv("PANE_SOCKET_PATH"); value != "" {
		return value, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return store.DefaultSocketPath(home), nil
}
