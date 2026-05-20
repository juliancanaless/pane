package daemon

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/juliancanalez/pane/internal/board"
	"github.com/juliancanalez/pane/internal/protocol"
	"github.com/juliancanalez/pane/internal/session"
	"github.com/juliancanalez/pane/internal/store"
)

type Config struct {
	SocketPath string
	DBPath     string
}

type Daemon struct {
	config  Config
	started time.Time
	db      *sql.DB
	manager session.Manager
}

func New(config Config) *Daemon {
	return &Daemon{config: config, started: time.Now()}
}

func NewForTest(config Config, manager session.Manager) *Daemon {
	return &Daemon{config: config, started: time.Now(), manager: manager}
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
		d.db = db
		d.manager = session.NewManager(store.NewSessionStore(db))
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
	case protocol.RequestSessionInit:
		return d.handleSessionInit(request)
	case protocol.RequestSessionStatus:
		return d.handleSessionStatus(request)
	case protocol.RequestSessionIntent:
		return d.handleSessionIntent(request)
	case protocol.RequestGetBoard:
		return d.handleGetBoard(request)
	default:
		return protocol.Failure(fmt.Sprintf("unsupported request type %q", request.Type))
	}
}

func (d *Daemon) handleSessionInit(request protocol.Request) protocol.Response {
	input := session.InitInput{
		PaneID:        payloadString(request, "pane_id"),
		TTY:           payloadString(request, "tty"),
		WorkspaceRoot: payloadString(request, "workspace_root"),
		CWD:           payloadString(request, "cwd"),
		Branch:        payloadString(request, "branch"),
	}
	result, err := d.manager.Init(context.Background(), input)
	if err != nil {
		return protocol.Failure(err.Error())
	}
	payload := sessionPayload(result.Session)
	payload["resumed"] = result.Resumed
	return protocol.Success(payload)
}

func (d *Daemon) handleSessionStatus(request protocol.Request) protocol.Response {
	current, err := d.manager.Status(context.Background(), payloadString(request, "pane_id"), payloadString(request, "workspace_root"))
	if errors.Is(err, session.ErrNotFound) {
		return protocol.Failure("no Pane session found for this pane/workspace; run `pane init` first")
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	return protocol.Success(sessionPayload(current))
}

func (d *Daemon) handleSessionIntent(request protocol.Request) protocol.Response {
	current, err := d.manager.Status(context.Background(), payloadString(request, "pane_id"), payloadString(request, "workspace_root"))
	if errors.Is(err, session.ErrNotFound) {
		return protocol.Failure("no Pane session found for this pane/workspace; run `pane init` first")
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	intent := payloadString(request, "intent")
	if err := d.manager.SetIntent(context.Background(), current.ID, intent); err != nil {
		return protocol.Failure(err.Error())
	}
	payload := sessionPayload(current)
	payload["intent"] = intent
	return protocol.Success(payload)
}

func (d *Daemon) handleGetBoard(request protocol.Request) protocol.Response {
	workspaceRoot := payloadString(request, "workspace_root")
	sessions, err := d.manager.ListActive(context.Background(), workspaceRoot)
	if err != nil {
		return protocol.Failure(err.Error())
	}
	text := board.Render(board.FromSessions(workspaceRoot, sessions), time.Now())
	return protocol.Success(map[string]any{"text": text})
}

func payloadString(request protocol.Request, key string) string {
	value, ok := request.Payload[key]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return fmt.Sprint(value)
	}
	return text
}

func sessionPayload(value session.Session) map[string]any {
	return map[string]any{
		"session_id":     value.ID,
		"pane_id":        value.PaneID,
		"tty":            value.TTY,
		"workspace_root": value.WorkspaceRoot,
		"cwd":            value.CWD,
		"branch":         value.Branch,
		"intent":         value.LastIntent,
		"started_at":     value.StartedAt,
		"last_seen_at":   value.LastSeenAt,
		"status":         string(value.Status),
	}
}
