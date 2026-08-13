package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/juliancanalez/pane/internal/daemon"
	"github.com/juliancanalez/pane/internal/protocol"
	"github.com/juliancanalez/pane/internal/store"
	"github.com/juliancanalez/pane/internal/version"
)

func sendDaemonRequest(request protocol.Request) (protocol.Response, error) {
	socket, err := socketPath()
	if err != nil {
		return protocol.Response{}, err
	}
	response, err := daemon.Client{SocketPath: socket}.Send(request)
	if err != nil {
		return protocol.Response{}, fmt.Errorf("Pane daemon is not running or is unreachable; start it with `pane daemon start`: %w", err)
	}
	maybeRestartStaleDaemon(socket, request.Type, response.DaemonVersion)
	return response, nil
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
	if err := daemon.Restart(socket); err != nil {
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
