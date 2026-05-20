package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/juliancanalez/pane/internal/daemon"
	"github.com/juliancanalez/pane/internal/protocol"
	"github.com/juliancanalez/pane/internal/store"
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
	return response, nil
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
