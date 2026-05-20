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
	"github.com/juliancanalez/pane/internal/messages"
	"github.com/juliancanalez/pane/internal/protocol"
	"github.com/juliancanalez/pane/internal/session"
	"github.com/juliancanalez/pane/internal/store"
	"github.com/juliancanalez/pane/internal/summary"
)

type Config struct {
	SocketPath string
	DBPath     string
}

type Daemon struct {
	config       Config
	started      time.Time
	db           *sql.DB
	manager      session.Manager
	messageStore store.MessageStore
}

func New(config Config) *Daemon {
	return &Daemon{config: config, started: time.Now()}
}

func NewForTest(config Config, manager session.Manager, messageStore store.MessageStore) *Daemon {
	return &Daemon{config: config, started: time.Now(), manager: manager, messageStore: messageStore}
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
		d.messageStore = store.NewMessageStore(db)
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
	case protocol.RequestGetSummary:
		return d.handleGetSummary(request)
	case protocol.RequestMessageSend:
		return d.handleMessageSend(request)
	case protocol.RequestMessageList:
		return d.handleMessageList(request)
	case protocol.RequestMessageReply:
		return d.handleMessageReply(request)
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

func (d *Daemon) handleGetSummary(request protocol.Request) protocol.Response {
	workspaceRoot := payloadString(request, "workspace_root")
	current, err := d.manager.Status(context.Background(), payloadString(request, "pane_id"), workspaceRoot)
	if errors.Is(err, session.ErrNotFound) {
		return protocol.Failure("no Pane session found for this pane/workspace; run `pane init` first")
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	sessions, err := d.manager.ListActive(context.Background(), workspaceRoot)
	if err != nil {
		return protocol.Failure(err.Error())
	}
	text := summary.Render(summary.FromSessions(workspaceRoot, current, sessions), time.Now())
	return protocol.Success(map[string]any{"text": text})
}

func (d *Daemon) handleMessageSend(request protocol.Request) protocol.Response {
	current, err := d.manager.Status(context.Background(), payloadString(request, "pane_id"), payloadString(request, "workspace_root"))
	if errors.Is(err, session.ErrNotFound) {
		return protocol.Failure("no Pane session found for this pane/workspace; run `pane init` first")
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	body := payloadString(request, "body")
	if body == "" {
		return protocol.Failure("message body cannot be empty")
	}
	message := messages.Message{
		ID:          messages.NewID(),
		FromSession: current.ID,
		ToSession:   payloadString(request, "to_session"),
		Body:        body,
		State:       messages.StateQueued,
		CreatedAt:   time.Now().Unix(),
	}
	if message.ToSession == "" {
		return protocol.Failure("target session cannot be empty")
	}
	message.ThreadID = message.ID
	if err := d.messageStore.Save(context.Background(), message); err != nil {
		return protocol.Failure(err.Error())
	}
	return protocol.Success(map[string]any{"message_id": message.ID, "thread_id": message.ThreadID, "to_session": message.ToSession})
}

func (d *Daemon) handleMessageList(request protocol.Request) protocol.Response {
	current, err := d.manager.Status(context.Background(), payloadString(request, "pane_id"), payloadString(request, "workspace_root"))
	if errors.Is(err, session.ErrNotFound) {
		return protocol.Failure("no Pane session found for this pane/workspace; run `pane init` first")
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	items, err := d.messageStore.ListQueuedForSession(context.Background(), current.ID)
	if err != nil {
		return protocol.Failure(err.Error())
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	if err := d.messageStore.MarkDelivered(context.Background(), ids, time.Now().Unix()); err != nil {
		return protocol.Failure(err.Error())
	}
	return protocol.Success(map[string]any{"text": messages.RenderInbox(items, time.Now()), "count": len(items)})
}

func (d *Daemon) handleMessageReply(request protocol.Request) protocol.Response {
	current, err := d.manager.Status(context.Background(), payloadString(request, "pane_id"), payloadString(request, "workspace_root"))
	if errors.Is(err, session.ErrNotFound) {
		return protocol.Failure("no Pane session found for this pane/workspace; run `pane init` first")
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	original, err := d.messageStore.FindByID(context.Background(), payloadString(request, "message_id"))
	if errors.Is(err, store.ErrNotFound) {
		return protocol.Failure("message not found")
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	body := payloadString(request, "body")
	if body == "" {
		return protocol.Failure("message body cannot be empty")
	}
	reply := messages.Message{
		ID:          messages.NewID(),
		ThreadID:    original.ThreadID,
		FromSession: current.ID,
		ToSession:   original.FromSession,
		Body:        body,
		State:       messages.StateQueued,
		CreatedAt:   time.Now().Unix(),
	}
	if err := d.messageStore.Save(context.Background(), reply); err != nil {
		return protocol.Failure(err.Error())
	}
	return protocol.Success(map[string]any{"message_id": reply.ID, "thread_id": reply.ThreadID, "to_session": reply.ToSession})
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
