package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/juliancanalez/pane/internal/protocol"
	"github.com/juliancanalez/pane/internal/store"
)

type Config struct {
	SocketPath string
	DBPath     string
}

type Daemon struct {
	config  Config
	started time.Time
}

func New(config Config) *Daemon {
	return &Daemon{config: config, started: time.Now()}
}

func (d *Daemon) Run(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(d.config.SocketPath), 0o755); err != nil {
		return err
	}
	if d.config.DBPath != "" {
		db, err := store.Open(d.config.DBPath)
		if err != nil {
			return err
		}
		defer db.Close()
	}
	if err := os.RemoveAll(d.config.SocketPath); err != nil {
		return err
	}

	listener, err := net.Listen("unix", d.config.SocketPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(d.config.SocketPath)
	}()

	stop := make(chan struct{})
	var stopOnce sync.Once
	requestStop := func() { stopOnce.Do(func() { close(stop) }) }

	go func() {
		select {
		case <-ctx.Done():
			requestStop()
		case <-stop:
		}
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-stop:
				return nil
			default:
			}
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		go d.handleConn(conn, requestStop)
	}
}

func (d *Daemon) handleConn(conn net.Conn, requestStop func()) {
	defer conn.Close()

	request, err := protocol.ReadMessage[protocol.Request](conn)
	if err != nil {
		_ = protocol.WriteMessage(conn, protocol.Failure(err.Error()))
		return
	}
	response := d.Handle(request, requestStop)
	_ = protocol.WriteMessage(conn, response)
}

func (d *Daemon) Handle(request protocol.Request, requestStop func()) protocol.Response {
	switch request.Type {
	case protocol.RequestDaemonHealth:
		return protocol.Success(map[string]any{
			"status":      "ok",
			"uptime_ms":   time.Since(d.started).Milliseconds(),
			"socket_path": d.config.SocketPath,
		})
	case protocol.RequestDaemonStop:
		requestStop()
		return protocol.Success(map[string]any{"status": "stopping"})
	default:
		return protocol.Failure(fmt.Sprintf("unsupported request type %q", request.Type))
	}
}
