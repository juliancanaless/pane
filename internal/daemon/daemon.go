package daemon

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/juliancanalez/pane/internal/activity"
	"github.com/juliancanalez/pane/internal/board"
	"github.com/juliancanalez/pane/internal/gitguard"
	"github.com/juliancanalez/pane/internal/messages"
	"github.com/juliancanalez/pane/internal/protocol"
	"github.com/juliancanalez/pane/internal/session"
	"github.com/juliancanalez/pane/internal/store"
	"github.com/juliancanalez/pane/internal/summary"
)

type Config struct {
	SocketPath string
	DBPath     string
	PIDPath    string
	LogPath    string
}

type Daemon struct {
	config        Config
	started       time.Time
	db            *sql.DB
	manager       session.Manager
	messageStore  store.MessageStore
	activityStore store.FileActivityStore
	gitEventStore store.GitEventStore
	stateStore    store.AgentStateStore
	watchers      map[string]context.CancelFunc
	watchersMu    sync.Mutex
}

func New(config Config) *Daemon {
	return &Daemon{config: config, started: time.Now(), watchers: make(map[string]context.CancelFunc)}
}

func NewForTest(config Config, manager session.Manager, messageStore store.MessageStore) *Daemon {
	return &Daemon{config: config, started: time.Now(), manager: manager, messageStore: messageStore, watchers: make(map[string]context.CancelFunc)}
}

func (d *Daemon) Run(ctx context.Context) error {
	if err := os.MkdirAll(filepath.Dir(d.config.SocketPath), 0o755); err != nil {
		return err
	}

	// Acquire exclusive process lock
	lockPath := strings.TrimSuffix(d.config.PIDPath, filepath.Ext(d.config.PIDPath)) + ".lock"
	if lockPath == ".lock" {
		lockPath = filepath.Join(filepath.Dir(d.config.SocketPath), "pane.lock")
	}
	lockFile, err := AcquireLock(lockPath)
	if err != nil {
		return err
	}
	defer ReleaseLock(lockFile)

	// Clean stale PID/socket from a previous crash
	if cleaned, err := CleanStale(d.config.PIDPath, d.config.SocketPath); err != nil {
		ReleaseLock(lockFile)
		return err
	} else if cleaned {
		fmt.Fprintf(os.Stderr, "cleaned stale daemon state\n")
	}

	// Set up log redirection and rotation
	if d.config.LogPath != "" {
		logFile, err := SetupLogging(d.config.LogPath)
		if err != nil {
			return fmt.Errorf("setup logging: %w", err)
		}
		if logFile != nil {
			defer logFile.Close()
		}
	}

	fmt.Fprintf(os.Stderr, "daemon starting on %s\n", d.config.SocketPath)
	if d.config.DBPath != "" {
		db, err := store.Open(d.config.DBPath)
		if err != nil {
			return err
		}
		d.db = db
		d.manager = session.NewManager(store.NewSessionStore(db))
		d.messageStore = store.NewMessageStore(db)
		d.activityStore = store.NewFileActivityStore(db)
		d.gitEventStore = store.NewGitEventStore(db)
		d.stateStore = store.NewAgentStateStore(db)
		defer db.Close()
	}
	if err := os.RemoveAll(d.config.SocketPath); err != nil {
		return err
	}

	listener, err := net.Listen("unix", d.config.SocketPath)
	if err != nil {
		return err
	}
	if err := d.writePIDFile(); err != nil {
		_ = listener.Close()
		return err
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(d.config.SocketPath)
		d.removePIDFile()
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

func (d *Daemon) writePIDFile() error {
	if d.config.PIDPath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(d.config.PIDPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(d.config.PIDPath, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o644)
}

func (d *Daemon) removePIDFile() {
	if d.config.PIDPath != "" {
		_ = os.Remove(d.config.PIDPath)
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
			"pid":         os.Getpid(),
			"uptime_ms":   time.Since(d.started).Milliseconds(),
			"socket_path": d.config.SocketPath,
			"db_path":     d.config.DBPath,
			"pid_path":    d.config.PIDPath,
			"log_path":    d.config.LogPath,
		})
	case protocol.RequestDaemonStop:
		requestStop()
		return protocol.Success(map[string]any{"status": "stopping"})
	case protocol.RequestSessionInit:
		return d.handleSessionInit(request)
	case protocol.RequestSessionHeartbeat:
		return d.handleSessionHeartbeat(request)
	case protocol.RequestSessionClose:
		return d.handleSessionClose(request)
	case protocol.RequestSessionPrune:
		return d.handleSessionPrune(request)
	case protocol.RequestSessionStatus:
		return d.handleSessionStatus(request)
	case protocol.RequestSessionIntent:
		return d.handleSessionIntent(request)
	case protocol.RequestSessionName:
		return d.handleSessionName(request)
	case protocol.RequestSessionContinue:
		return d.handleSessionContinue(request)
	case protocol.RequestSessionHistory:
		return d.handleSessionHistory(request)
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
	case protocol.RequestGitPreflight:
		return d.handleGitPreflight(request)
	case protocol.RequestGitRecord:
		return d.handleGitRecord(request)
	case protocol.RequestStateSet:
		return d.handleStateSet(request)
	case protocol.RequestStateGet:
		return d.handleStateGet(request)
	case protocol.RequestStateList:
		return d.handleStateList(request)
	case protocol.RequestStateDelete:
		return d.handleStateDelete(request)
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
	d.ensureWorkspaceWatcher(input.WorkspaceRoot)
	payload := sessionPayload(result.Session)
	payload["resumed"] = result.Resumed
	return protocol.Success(payload)
}

func (d *Daemon) handleSessionHeartbeat(request protocol.Request) protocol.Response {
	input := session.InitInput{
		PaneID:        payloadString(request, "pane_id"),
		TTY:           payloadString(request, "tty"),
		WorkspaceRoot: payloadString(request, "workspace_root"),
		CWD:           payloadString(request, "cwd"),
		Branch:        payloadString(request, "branch"),
	}
	result, err := d.manager.Heartbeat(context.Background(), input)
	if err != nil {
		return protocol.Failure(err.Error())
	}
	d.ensureWorkspaceWatcher(input.WorkspaceRoot)
	payload := sessionPayload(result.Session)
	payload["resumed"] = result.Resumed
	return protocol.Success(payload)
}

func (d *Daemon) handleSessionClose(request protocol.Request) protocol.Response {
	current, err := d.manager.Close(context.Background(), payloadString(request, "pane_id"), payloadString(request, "workspace_root"))
	if errors.Is(err, session.ErrNotFound) {
		return protocol.Failure("no Pane session found for this pane/workspace; run `pane init` first")
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	return protocol.Success(sessionPayload(current))
}

func (d *Daemon) handleSessionPrune(request protocol.Request) protocol.Response {
	count, err := d.manager.PruneStale(context.Background(), payloadString(request, "workspace_root"))
	if err != nil {
		return protocol.Failure(err.Error())
	}
	return protocol.Success(map[string]any{"closed": count})
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

func (d *Daemon) handleSessionContinue(request protocol.Request) protocol.Response {
	input := session.InitInput{
		PaneID:        payloadString(request, "pane_id"),
		TTY:           payloadString(request, "tty"),
		WorkspaceRoot: payloadString(request, "workspace_root"),
		CWD:           payloadString(request, "cwd"),
		Branch:        payloadString(request, "branch"),
	}
	parent, err := d.manager.Resolve(context.Background(), input.WorkspaceRoot, payloadString(request, "parent_session_id"))
	if errors.Is(err, session.ErrNotFound) {
		return protocol.Failure("session to continue was not found")
	}
	if errors.Is(err, session.ErrAmbiguous) {
		return protocol.Failure(err.Error())
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	result, err := d.manager.Continue(context.Background(), input, parent.ID)
	if err != nil {
		return protocol.Failure(err.Error())
	}
	d.ensureWorkspaceWatcher(input.WorkspaceRoot)
	payload := sessionPayload(result.Session)
	payload["resumed"] = result.Resumed
	return protocol.Success(payload)
}

func (d *Daemon) handleSessionHistory(request protocol.Request) protocol.Response {
	workspaceRoot := payloadString(request, "workspace_root")
	items, err := d.manager.ListRecent(context.Background(), workspaceRoot, 100)
	if err != nil {
		return protocol.Failure(err.Error())
	}
	items = filterSince(items, payloadInt64(request, "since"))
	if len(items) > 20 {
		items = items[:20]
	}
	return protocol.Success(map[string]any{"text": renderHistory(workspaceRoot, items, time.Now())})
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

func (d *Daemon) handleSessionName(request protocol.Request) protocol.Response {
	current, err := d.manager.Status(context.Background(), payloadString(request, "pane_id"), payloadString(request, "workspace_root"))
	if errors.Is(err, session.ErrNotFound) {
		return protocol.Failure("no Pane session found for this pane/workspace; run `pane init` first")
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	name := payloadString(request, "name")
	if err := d.manager.SetName(context.Background(), current.ID, name); err != nil {
		return protocol.Failure(err.Error())
	}
	payload := sessionPayload(current)
	payload["name"] = name
	return protocol.Success(payload)
}

func (d *Daemon) handleGetBoard(request protocol.Request) protocol.Response {
	workspaceRoot := payloadString(request, "workspace_root")
	var sessions []session.Session
	var err error
	if payloadBool(request, "show_all") {
		sessions, err = d.manager.ListRecent(context.Background(), workspaceRoot, 50)
	} else {
		sessions, err = d.manager.ListActive(context.Background(), workspaceRoot)
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	stats, err := d.boardMessageStats(context.Background(), sessions)
	if err != nil {
		return protocol.Failure(err.Error())
	}
	activityStats, err := d.boardActivityStats(context.Background(), sessions)
	if err != nil {
		return protocol.Failure(err.Error())
	}
	b := board.FromSessionsWithStats(workspaceRoot, sessions, stats, activityStats)
	b.Overlaps = d.boardOverlaps(context.Background(), workspaceRoot)
	b.RecentGitEvents = d.boardGitEvents(context.Background(), workspaceRoot, sessions)
	text := board.Render(b, time.Now())
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
	unread, err := d.messageStore.ListQueuedForSession(context.Background(), current.ID)
	if err != nil {
		return protocol.Failure(err.Error())
	}
	awaiting, err := d.messageStore.CountOpenOutboundForSession(context.Background(), current.ID)
	if err != nil {
		return protocol.Failure(err.Error())
	}
	recentActivity, err := d.activityStore.RecentBySession(context.Background(), current.ID, time.Now().Add(-15*time.Minute).Unix(), 5)
	if err != nil {
		return protocol.Failure(err.Error())
	}
	coordination := summary.Coordination{UnreadMessages: unread, AwaitingReplies: awaiting}
	lineage := d.summaryLineage(context.Background(), workspaceRoot, current)
	s := summary.FromSessionsWithLineage(workspaceRoot, current, sessions, coordination, activity.RecentFiles(recentActivity, 5), lineage)
	s.Overlaps = d.summaryOverlaps(context.Background(), workspaceRoot, current.ID, sessions)
	text := summary.Render(s, time.Now())
	return protocol.Success(map[string]any{"text": text})
}

func (d *Daemon) ensureWorkspaceWatcher(workspaceRoot string) {
	if workspaceRoot == "" || d.activityStore == (store.FileActivityStore{}) {
		return
	}
	d.watchersMu.Lock()
	defer d.watchersMu.Unlock()
	if _, ok := d.watchers[workspaceRoot]; ok {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	d.watchers[workspaceRoot] = cancel
	watcher := activity.PollWatcher{
		Root: workspaceRoot,
		OnEvent: func(event activity.WatchEvent) {
			d.recordWatchEvent(workspaceRoot, event)
		},
	}
	go func() { _ = watcher.Run(ctx) }()
}

func (d *Daemon) recordWatchEvent(workspaceRoot string, event activity.WatchEvent) {
	sessions, err := d.manager.ListActive(context.Background(), workspaceRoot)
	if err != nil || len(sessions) == 0 {
		return
	}
	owner, attribution := attributeSession(sessions, event.Path)
	_ = d.activityStore.Save(context.Background(), activity.FileActivity{
		SessionID:   owner.ID,
		Path:        event.Path,
		EventType:   event.EventType,
		Attribution: attribution,
		Timestamp:   event.Time.Unix(),
	})
}

func attributeSession(sessions []session.Session, path string) (session.Session, activity.Attribution) {
	if len(sessions) == 1 {
		return sessions[0], activity.AttributionHigh
	}
	best := sessions[0]
	bestLen := -1
	for _, item := range sessions {
		if item.CWD != "" && pathHasPrefix(path, item.CWD) && len(item.CWD) > bestLen {
			best = item
			bestLen = len(item.CWD)
		}
	}
	if bestLen >= 0 {
		return best, activity.AttributionMedium
	}
	for _, item := range sessions[1:] {
		if item.LastSeenAt > best.LastSeenAt {
			best = item
		}
	}
	return best, activity.AttributionLow
}

func pathHasPrefix(path, prefix string) bool {
	relative, err := filepath.Rel(prefix, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, "../")
}

func (d *Daemon) boardOverlaps(ctx context.Context, workspaceRoot string) []board.OverlapInfo {
	if d.activityStore == (store.FileActivityStore{}) {
		return nil
	}
	pathSessions, err := d.activityStore.OverlapByWorkspace(ctx, workspaceRoot, time.Now().Add(-15*time.Minute).Unix())
	if err != nil || len(pathSessions) == 0 {
		return nil
	}
	overlaps := activity.ComputeOverlap(pathSessions)
	result := make([]board.OverlapInfo, 0, len(overlaps))
	for _, o := range overlaps {
		result = append(result, board.OverlapInfo{
			SessionA:    o.SessionA,
			SessionB:    o.SessionB,
			SharedFiles: o.SharedFiles,
		})
	}
	return result
}

func (d *Daemon) boardGitEvents(ctx context.Context, workspaceRoot string, sessions []session.Session) []board.GitEventInfo {
	if d.gitEventStore == (store.GitEventStore{}) {
		return nil
	}
	events, err := d.gitEventStore.RecentByWorkspace(ctx, workspaceRoot, time.Now().Add(-15*time.Minute).Unix(), 5)
	if err != nil || len(events) == 0 {
		return nil
	}
	// Build session name lookup
	nameMap := make(map[string]string, len(sessions))
	for _, s := range sessions {
		if s.Name != "" {
			nameMap[s.ID] = s.Name
		}
	}
	result := make([]board.GitEventInfo, 0, len(events))
	for _, e := range events {
		cmd := e.Subcommand
		if cmd == "" {
			cmd = e.Command
		}
		result = append(result, board.GitEventInfo{
			SessionShortID: session.ShortID(e.SessionID),
			SessionName:    nameMap[e.SessionID],
			Command:        cmd,
			Timestamp:      e.Timestamp,
		})
	}
	return result
}

func (d *Daemon) summaryOverlaps(ctx context.Context, workspaceRoot, currentSessionID string, sessions []session.Session) []summary.OverlapInfo {
	if d.activityStore == (store.FileActivityStore{}) {
		return nil
	}
	pathSessions, err := d.activityStore.OverlapByWorkspace(ctx, workspaceRoot, time.Now().Add(-15*time.Minute).Unix())
	if err != nil || len(pathSessions) == 0 {
		return nil
	}
	nameMap := make(map[string]string, len(sessions))
	for _, s := range sessions {
		if s.Name != "" {
			nameMap[s.ID] = s.Name
		}
	}
	overlaps := activity.ComputeOverlap(pathSessions)
	var result []summary.OverlapInfo
	for _, o := range overlaps {
		if o.SessionA == currentSessionID {
			result = append(result, summary.OverlapInfo{PeerSessionID: o.SessionB, PeerName: nameMap[o.SessionB], SharedFiles: o.SharedFiles})
		} else if o.SessionB == currentSessionID {
			result = append(result, summary.OverlapInfo{PeerSessionID: o.SessionA, PeerName: nameMap[o.SessionA], SharedFiles: o.SharedFiles})
		}
	}
	return result
}

func (d *Daemon) gitPreflightOverlaps(ctx context.Context, workspaceRoot, currentSessionID string) []gitguard.FileOverlap {
	if d.activityStore == (store.FileActivityStore{}) {
		return nil
	}
	pathSessions, err := d.activityStore.OverlapByWorkspace(ctx, workspaceRoot, time.Now().Add(-15*time.Minute).Unix())
	if err != nil || len(pathSessions) == 0 {
		return nil
	}
	overlaps := activity.ComputeOverlap(pathSessions)
	var result []gitguard.FileOverlap
	for _, o := range overlaps {
		if o.SessionA == currentSessionID {
			result = append(result, gitguard.FileOverlap{PeerSessionID: o.SessionB, SharedFiles: o.SharedFiles})
		} else if o.SessionB == currentSessionID {
			result = append(result, gitguard.FileOverlap{PeerSessionID: o.SessionA, SharedFiles: o.SharedFiles})
		}
	}
	return result
}

func (d *Daemon) boardActivityStats(ctx context.Context, sessions []session.Session) (map[string]board.ActivityStats, error) {
	stats := make(map[string]board.ActivityStats, len(sessions))
	for _, item := range sessions {
		recent, err := d.activityStore.RecentBySession(ctx, item.ID, time.Now().Add(-15*time.Minute).Unix(), 10)
		if err != nil {
			return nil, err
		}
		files := activity.RecentFiles(recent, 5)
		stats[item.ID] = board.ActivityStats{
			RecentFiles:    files[:min(len(files), 3)],
			HotDirectories: activity.HotDirectories(activity.RecentFiles(recent, 10), 3),
		}
	}
	return stats, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (d *Daemon) boardMessageStats(ctx context.Context, sessions []session.Session) (map[string]board.MessageStats, error) {
	stats := make(map[string]board.MessageStats, len(sessions))
	for _, item := range sessions {
		unread, err := d.messageStore.CountQueuedForSession(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		awaiting, err := d.messageStore.CountOpenOutboundForSession(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		stats[item.ID] = board.MessageStats{UnreadMessages: unread, AwaitingReplies: awaiting}
	}
	return stats, nil
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
	targetRef := payloadString(request, "to_session")
	if targetRef == "" {
		return protocol.Failure("target session cannot be empty")
	}
	target, err := d.manager.Resolve(context.Background(), current.WorkspaceRoot, targetRef)
	if errors.Is(err, session.ErrNotFound) {
		return protocol.Failure("target session not found")
	}
	if errors.Is(err, session.ErrAmbiguous) {
		return protocol.Failure(err.Error())
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	message := messages.Message{
		ID:          messages.NewID(),
		FromSession: current.ID,
		ToSession:   target.ID,
		Body:        body,
		State:       messages.StateQueued,
		CreatedAt:   time.Now().Unix(),
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

func (d *Daemon) handleStateSet(request protocol.Request) protocol.Response {
	current, err := d.manager.Status(context.Background(), payloadString(request, "pane_id"), payloadString(request, "workspace_root"))
	if errors.Is(err, session.ErrNotFound) {
		return protocol.Failure("no Pane session found for this pane/workspace; run `pane init` first")
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	key := payloadString(request, "key")
	if key == "" {
		return protocol.Failure("state key cannot be empty")
	}
	valueJSON := payloadString(request, "value_json")
	if valueJSON == "" {
		return protocol.Failure("state value cannot be empty")
	}
	item := store.AgentState{WorkspaceRoot: current.WorkspaceRoot, Key: key, ValueJSON: valueJSON, UpdatedAt: time.Now().Unix(), SessionID: current.ID}
	if err := d.stateStore.Set(context.Background(), item); err != nil {
		return protocol.Failure(err.Error())
	}
	return protocol.Success(map[string]any{"key": key})
}

func (d *Daemon) handleStateGet(request protocol.Request) protocol.Response {
	workspaceRoot := payloadString(request, "workspace_root")
	key := payloadString(request, "key")
	if key == "" {
		return protocol.Failure("state key cannot be empty")
	}
	item, err := d.stateStore.Get(context.Background(), workspaceRoot, key)
	if errors.Is(err, store.ErrNotFound) {
		return protocol.Failure("state key not found")
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	return protocol.Success(map[string]any{"key": item.Key, "value_json": item.ValueJSON, "updated_at": item.UpdatedAt, "session_id": item.SessionID})
}

func (d *Daemon) handleStateList(request protocol.Request) protocol.Response {
	workspaceRoot := payloadString(request, "workspace_root")
	items, err := d.stateStore.List(context.Background(), workspaceRoot, payloadString(request, "prefix"))
	if err != nil {
		return protocol.Failure(err.Error())
	}
	return protocol.Success(map[string]any{"items": stateItemsPayload(items)})
}

func (d *Daemon) handleStateDelete(request protocol.Request) protocol.Response {
	workspaceRoot := payloadString(request, "workspace_root")
	key := payloadString(request, "key")
	if key == "" {
		return protocol.Failure("state key cannot be empty")
	}
	if err := d.stateStore.Delete(context.Background(), workspaceRoot, key); errors.Is(err, store.ErrNotFound) {
		return protocol.Failure("state key not found")
	} else if err != nil {
		return protocol.Failure(err.Error())
	}
	return protocol.Success(map[string]any{"key": key})
}

func stateItemsPayload(items []store.AgentState) []map[string]any {
	payload := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload = append(payload, map[string]any{"key": item.Key, "value_json": item.ValueJSON, "updated_at": item.UpdatedAt, "session_id": item.SessionID})
	}
	return payload
}

func (d *Daemon) handleGitPreflight(request protocol.Request) protocol.Response {
	args := payloadStrings(request, "args")
	intent := gitguard.Parse(args)
	if !intent.Watched {
		return protocol.Success(nil)
	}
	current, err := d.manager.Status(context.Background(), payloadString(request, "pane_id"), payloadString(request, "workspace_root"))
	if errors.Is(err, session.ErrNotFound) {
		return protocol.Success(nil)
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	sessions, err := d.manager.ListActive(context.Background(), payloadString(request, "workspace_root"))
	if err != nil {
		return protocol.Failure(err.Error())
	}
	result := gitguard.Preflight(gitguard.PreflightInput{
		Intent:         intent,
		CurrentSession: current,
		ActiveSessions: sessions,
		FileOverlaps:   d.gitPreflightOverlaps(context.Background(), current.WorkspaceRoot, current.ID),
	})
	return protocol.Response{OK: true, Warnings: result.Warnings, Block: result.Block}
}

func (d *Daemon) handleGitRecord(request protocol.Request) protocol.Response {
	current, err := d.manager.Status(context.Background(), payloadString(request, "pane_id"), payloadString(request, "workspace_root"))
	if errors.Is(err, session.ErrNotFound) {
		return protocol.Success(nil)
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	args := payloadStrings(request, "args")
	intent := gitguard.Parse(args)
	if len(args) == 0 {
		return protocol.Success(nil)
	}
	event := store.GitEvent{
		SessionID:    current.ID,
		Command:      strings.Join(args, " "),
		Subcommand:   intent.Subcommand,
		Branch:       payloadString(request, "branch"),
		TargetBranch: intent.TargetBranch,
		Timestamp:    time.Now().Unix(),
		Result:       payloadString(request, "result"),
	}
	if err := d.gitEventStore.Save(context.Background(), event); err != nil {
		return protocol.Failure(err.Error())
	}
	return protocol.Success(nil)
}

func (d *Daemon) summaryLineage(ctx context.Context, workspaceRoot string, current session.Session) summary.Lineage {
	items, err := d.manager.ListRecent(ctx, workspaceRoot, 5)
	if err != nil {
		return summary.Lineage{}
	}
	lineage := summary.Lineage{History: summary.HistoryFromSessions(filterSession(items, current.ID))}
	if current.ParentID != "" {
		for _, item := range items {
			if item.ID == current.ParentID {
				line := summary.HistoryFromSessions([]session.Session{item})[0]
				lineage.Parent = &line
				break
			}
		}
	}
	return lineage
}

func filterSession(items []session.Session, sessionID string) []session.Session {
	filtered := make([]session.Session, 0, len(items))
	for _, item := range items {
		if item.ID != sessionID {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterSince(items []session.Session, since int64) []session.Session {
	if since <= 0 {
		return items
	}
	filtered := make([]session.Session, 0, len(items))
	for _, item := range items {
		if item.LastSeenAt >= since {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func renderHistory(workspaceRoot string, items []session.Session, now time.Time) string {
	var out strings.Builder
	fmt.Fprintf(&out, "[Pane] Session history\n")
	fmt.Fprintf(&out, "Workspace: %s\n", workspaceRoot)
	if len(items) == 0 {
		fmt.Fprintf(&out, "No sessions recorded.\n")
		return out.String()
	}
	for _, item := range items {
		shortID := session.ShortID(item.ID)
		fmt.Fprintf(&out, "\n%s", item.ID)
		if item.Name != "" {
			fmt.Fprintf(&out, " (%s)", item.Name)
		} else {
			fmt.Fprintf(&out, " (short: %s)", shortID)
		}
		fmt.Fprintf(&out, " — %s", statusLabel(item.Status))
		if item.Branch != "" {
			fmt.Fprintf(&out, " — %s", item.Branch)
		}
		if item.ParentID != "" {
			fmt.Fprintf(&out, " — continued from %s", session.ShortID(item.ParentID))
		}
		fmt.Fprintf(&out, "\n  Intent: %s\n", displayText(item.LastIntent, "not set"))
		fmt.Fprintf(&out, "  CWD: %s\n", displayText(item.CWD, "unknown"))
		fmt.Fprintf(&out, "  Last seen: %s\n", relativeTime(item.LastSeenAt, now))
	}
	return out.String()
}

func statusLabel(status session.Status) string {
	switch status {
	case session.StatusActive:
		return "🟢 active"
	case session.StatusIdle:
		return "🟡 idle"
	case session.StatusClosed:
		return "⚫ closed"
	default:
		return string(status)
	}
}

func displayText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func relativeTime(timestamp int64, now time.Time) string {
	if timestamp <= 0 {
		return "unknown"
	}
	delta := now.Sub(time.Unix(timestamp, 0))
	if delta < 0 {
		delta = 0
	}
	if delta < time.Minute {
		return fmt.Sprintf("%ds ago", int(delta.Seconds()))
	}
	if delta < time.Hour {
		return fmt.Sprintf("%dm ago", int(delta.Minutes()))
	}
	if delta < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(delta.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(delta.Hours()/24))
}

func payloadStrings(request protocol.Request, key string) []string {
	value, ok := request.Payload[key]
	if !ok || value == nil {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			items = append(items, fmt.Sprint(item))
		}
		return items
	default:
		return nil
	}
}

func payloadInt64(request protocol.Request, key string) int64 {
	value, ok := request.Payload[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
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

func payloadBool(request protocol.Request, key string) bool {
	value, ok := request.Payload[key]
	if !ok || value == nil {
		return false
	}
	b, ok := value.(bool)
	return ok && b
}

func sessionPayload(value session.Session) map[string]any {
	return map[string]any{
		"session_id":        value.ID,
		"pane_id":           value.PaneID,
		"tty":               value.TTY,
		"workspace_root":    value.WorkspaceRoot,
		"cwd":               value.CWD,
		"branch":            value.Branch,
		"intent":            value.LastIntent,
		"started_at":        value.StartedAt,
		"last_seen_at":      value.LastSeenAt,
		"status":            string(value.Status),
		"parent_session_id": value.ParentID,
	}
}
