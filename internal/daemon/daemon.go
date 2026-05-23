package daemon

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/juliancanalez/pane/internal/activity"
	"github.com/juliancanalez/pane/internal/analysis"
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
	analysisStore store.AnalysisStore
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
		d.analysisStore = store.NewAnalysisStore(db)
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
		PaneID:          payloadString(request, "pane_id"),
		TTY:             payloadString(request, "tty"),
		WorkspaceRoot:   payloadString(request, "workspace_root"),
		CWD:             payloadString(request, "cwd"),
		Branch:          payloadString(request, "branch"),
		RepoID:          payloadString(request, "repo_id"),
		GitCommonDir:    payloadString(request, "git_common_dir"),
		ParentSessionID: payloadString(request, "parent_session_id"),
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
		PaneID:          payloadString(request, "pane_id"),
		TTY:             payloadString(request, "tty"),
		WorkspaceRoot:   payloadString(request, "workspace_root"),
		CWD:             payloadString(request, "cwd"),
		Branch:          payloadString(request, "branch"),
		RepoID:          payloadString(request, "repo_id"),
		GitCommonDir:    payloadString(request, "git_common_dir"),
		ParentSessionID: payloadString(request, "parent_session_id"),
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
		RepoID:        payloadString(request, "repo_id"),
		GitCommonDir:  payloadString(request, "git_common_dir"),
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
	repoID := payloadString(request, "repo_id")
	var items []session.Session
	var err error
	if payloadString(request, "scope") == "repo" && repoID != "" {
		items, err = d.manager.ListRecentByRepo(context.Background(), repoID, 100)
	} else {
		items, err = d.manager.ListRecent(context.Background(), workspaceRoot, 100)
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	items = filterSince(items, payloadInt64(request, "since"))
	if len(items) > 20 {
		items = items[:20]
	}
	activitySummaries := d.historyActivitySummaries(context.Background(), items)
	if payloadString(request, "format") == "work-log" {
		return protocol.Success(map[string]any{"text": d.renderWorkLog(context.Background(), workspaceRoot, items, payloadInt64(request, "since"), time.Now())})
	}
	return protocol.Success(map[string]any{"text": renderHistory(workspaceRoot, items, activitySummaries, payloadBool(request, "lineage"), time.Now())})
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
	repoID := payloadString(request, "repo_id")
	scope := payloadString(request, "scope")
	var sessions []session.Session
	var err error
	if scope == "repo" && repoID != "" {
		if payloadBool(request, "show_all") {
			sessions, err = d.manager.ListRecentByRepo(context.Background(), repoID, 50)
		} else {
			sessions, err = d.manager.ListActiveByRepo(context.Background(), repoID)
		}
	} else if payloadBool(request, "show_all") {
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
	b.Scope = scope
	b.RepoID = repoID
	b.Overlaps = d.boardOverlaps(context.Background(), workspaceRoot)
	b.SemanticOverlaps = d.boardSemanticOverlaps(context.Background(), workspaceRoot, sessions)
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
	recentActivity, err := d.activityStore.RecentBySession(context.Background(), current.ID, time.Now().Add(-activity.CompressedWindow).Unix(), 100)
	if err != nil {
		return protocol.Failure(err.Error())
	}
	activityDigest := activity.DecayActivities(recentActivity, time.Now(), 5, 3)
	coordination := summary.Coordination{UnreadMessages: unread, AwaitingReplies: awaiting}
	lineage := d.summaryLineage(context.Background(), workspaceRoot, current)
	s := summary.FromSessionsWithLineage(workspaceRoot, current, sessions, coordination, activityDigest.FullFiles, lineage)
	s.ActivitySummaries = activityDigest.Lines()
	s.StateItems = d.summaryStateItems(context.Background(), workspaceRoot)
	s.Overlaps = d.summaryOverlaps(context.Background(), workspaceRoot, current.ID, sessions)
	s.SemanticOverlaps = d.summarySemanticOverlaps(context.Background(), workspaceRoot, current.ID, sessions)
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
	watcher := activity.NativeWatcher{
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
	// Store paths relative to workspace root for consistent overlap matching
	path := event.Path
	if rel, err := filepath.Rel(workspaceRoot, path); err == nil && !strings.HasPrefix(rel, "..") {
		path = rel
	}
	d.indexAnalysisFile(workspaceRoot, path)
	for _, owner := range attributeSessions(sessions, event.Path) {
		_ = d.activityStore.Save(context.Background(), activity.FileActivity{
			SessionID:   owner.Session.ID,
			Path:        path,
			EventType:   event.EventType,
			Attribution: owner.Attribution,
			Timestamp:   event.Time.Unix(),
		})
	}
}

func (d *Daemon) indexAnalysisFile(workspaceRoot, relativePath string) {
	if d.analysisStore == (store.AnalysisStore{}) || !isSupportedAnalysisPath(relativePath) {
		return
	}
	file := filepath.Join(workspaceRoot, relativePath)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client := analysis.Client{}
		table, err := client.Symbols(ctx, file)
		if err != nil {
			return
		}
		graph, err := client.Dependencies(ctx, file)
		if err != nil {
			return
		}
		_ = d.analysisStore.UpsertFile(context.Background(), store.FileAnalysis{
			WorkspaceRoot: workspaceRoot,
			File:          filepath.ToSlash(relativePath),
			Language:      table.Language,
			Symbols:       analysisSymbolsForStore(table.Symbols),
			Dependencies:  analysisDependenciesForStore(graph.Dependencies),
		})
	}()
}

func isSupportedAnalysisPath(path string) bool {
	switch filepath.Ext(path) {
	case ".go", ".py", ".rs", ".ts", ".tsx":
		return true
	default:
		return false
	}
}

func analysisSymbolsForStore(symbols []analysis.Symbol) []store.AnalysisSymbol {
	out := make([]store.AnalysisSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		out = append(out, store.AnalysisSymbol{Name: symbol.Name, Kind: symbol.Kind, StartLine: symbol.StartLine, EndLine: symbol.EndLine})
	}
	return out
}

func analysisDependenciesForStore(dependencies []analysis.Dependency) []store.DependencyEdge {
	out := make([]store.DependencyEdge, 0, len(dependencies))
	for _, dep := range dependencies {
		out = append(out, store.DependencyEdge{Target: dep.Target, TargetSymbol: dep.TargetSymbol, Kind: dep.Kind, Confidence: dep.Confidence})
	}
	return out
}

type attributedSession struct {
	Session     session.Session
	Attribution activity.Attribution
}

func attributeSessions(sessions []session.Session, path string) []attributedSession {
	if len(sessions) == 0 {
		return nil
	}
	if len(sessions) == 1 {
		return []attributedSession{{Session: sessions[0], Attribution: activity.AttributionHigh}}
	}

	bestLen := -1
	var best []session.Session
	for _, item := range sessions {
		if item.CWD == "" || !pathHasPrefix(path, item.CWD) {
			continue
		}
		if len(item.CWD) > bestLen {
			bestLen = len(item.CWD)
			best = []session.Session{item}
			continue
		}
		if len(item.CWD) == bestLen {
			best = append(best, item)
		}
	}
	if bestLen >= 0 {
		attribution := activity.AttributionMedium
		if len(best) > 1 {
			attribution = activity.AttributionLow
		}
		return attributedSessions(best, attribution)
	}

	latest := sessions[0]
	for _, item := range sessions[1:] {
		if item.LastSeenAt > latest.LastSeenAt {
			latest = item
		}
	}
	return []attributedSession{{Session: latest, Attribution: activity.AttributionLow}}
}

func attributedSessions(sessions []session.Session, attribution activity.Attribution) []attributedSession {
	owners := make([]attributedSession, 0, len(sessions))
	for _, item := range sessions {
		owners = append(owners, attributedSession{Session: item, Attribution: attribution})
	}
	return owners
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

func (d *Daemon) summaryStateItems(ctx context.Context, workspaceRoot string) []summary.StateItem {
	if d.stateStore == (store.AgentStateStore{}) {
		return nil
	}
	items, err := d.stateStore.List(ctx, workspaceRoot, "summary.")
	if err != nil {
		return nil
	}
	result := make([]summary.StateItem, 0, len(items))
	for _, item := range items {
		result = append(result, summary.StateItem{Key: item.Key, ValueJSON: item.ValueJSON, SessionID: item.SessionID, UpdatedAt: item.UpdatedAt})
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

func (d *Daemon) boardSemanticOverlaps(ctx context.Context, workspaceRoot string, sessions []session.Session) []board.SemanticOverlapInfo {
	impacts := d.semanticImpacts(ctx, workspaceRoot, sessions)
	result := make([]board.SemanticOverlapInfo, 0, len(impacts))
	for _, impact := range impacts {
		result = append(result, board.SemanticOverlapInfo{
			SourceSession:    impact.SourceSession,
			DependentSession: impact.DependentSession,
			ChangedFile:      impact.ChangedFile,
			DependentFile:    impact.DependentFile,
			Symbol:           impact.Symbol,
			Dependency:       impact.Dependency,
			Confidence:       impact.Confidence,
		})
	}
	return result
}

func (d *Daemon) summarySemanticOverlaps(ctx context.Context, workspaceRoot, currentSessionID string, sessions []session.Session) []summary.SemanticOverlapInfo {
	impacts := d.semanticImpacts(ctx, workspaceRoot, sessions)
	nameMap := sessionNameMap(sessions)
	var result []summary.SemanticOverlapInfo
	for _, impact := range impacts {
		if impact.SourceSession == currentSessionID {
			result = append(result, summary.SemanticOverlapInfo{PeerSessionID: impact.DependentSession, PeerName: nameMap[impact.DependentSession], ChangedFile: impact.ChangedFile, DependentFile: impact.DependentFile, Symbol: impact.Symbol, Dependency: impact.Dependency, Confidence: impact.Confidence})
		} else if impact.DependentSession == currentSessionID {
			result = append(result, summary.SemanticOverlapInfo{PeerSessionID: impact.SourceSession, PeerName: nameMap[impact.SourceSession], ChangedFile: impact.ChangedFile, DependentFile: impact.DependentFile, Symbol: impact.Symbol, Dependency: impact.Dependency, Confidence: impact.Confidence})
		}
	}
	return result
}

func (d *Daemon) gitPreflightSemanticOverlaps(ctx context.Context, workspaceRoot, currentSessionID string, sessions []session.Session) []gitguard.SemanticOverlap {
	impacts := d.semanticImpacts(ctx, workspaceRoot, sessions)
	var result []gitguard.SemanticOverlap
	for _, impact := range impacts {
		if impact.SourceSession == currentSessionID {
			result = append(result, gitguard.SemanticOverlap{PeerSessionID: impact.DependentSession, ChangedFile: impact.ChangedFile, DependentFile: impact.DependentFile, Symbol: impact.Symbol, Dependency: impact.Dependency, Confidence: impact.Confidence})
		}
	}
	return result
}

type semanticImpact struct {
	SourceSession    string
	DependentSession string
	ChangedFile      string
	DependentFile    string
	Symbol           string
	Dependency       string
	Confidence       float64
}

func (d *Daemon) semanticImpacts(ctx context.Context, workspaceRoot string, sessions []session.Session) []semanticImpact {
	if d.activityStore == (store.FileActivityStore{}) || d.analysisStore == (store.AnalysisStore{}) {
		return nil
	}
	recent, err := d.activityStore.RecentByWorkspace(ctx, workspaceRoot, time.Now().Add(-15*time.Minute).Unix(), 250)
	if err != nil || len(recent) == 0 {
		return nil
	}
	sessionFiles := recentFilesBySession(workspaceRoot, recent, activeSessionIDs(sessions))
	allFiles := uniqueFiles(sessionFiles)
	symbolsByFile, err := d.analysisStore.SymbolsByFile(ctx, workspaceRoot, allFiles)
	if err != nil {
		return nil
	}
	if len(symbolsByFile) == 0 {
		return nil
	}

	var impacts []semanticImpact
	seen := make(map[string]bool)
	for sourceSession, changedFiles := range sessionFiles {
		for dependentSession, dependentFiles := range sessionFiles {
			if sourceSession == dependentSession {
				continue
			}
			edges, err := d.analysisStore.EdgesBySourceFiles(ctx, workspaceRoot, dependentFiles)
			if err != nil || len(edges) == 0 {
				continue
			}
			for _, changedFile := range changedFiles {
				matches := matchingImpacts(sourceSession, dependentSession, changedFile, symbolsByFile[changedFile], edges)
				for _, impact := range matches {
					key := strings.Join([]string{impact.SourceSession, impact.DependentSession, impact.ChangedFile, impact.DependentFile, impact.Symbol, impact.Dependency}, "\x00")
					if seen[key] {
						continue
					}
					seen[key] = true
					impacts = append(impacts, impact)
				}
			}
		}
	}
	sort.Slice(impacts, func(i, j int) bool {
		if impacts[i].Confidence != impacts[j].Confidence {
			return impacts[i].Confidence > impacts[j].Confidence
		}
		return impacts[i].ChangedFile < impacts[j].ChangedFile
	})
	if len(impacts) > 10 {
		return impacts[:10]
	}
	return impacts
}

func recentFilesBySession(workspaceRoot string, recent []activity.FileActivity, activeSessions map[string]bool) map[string][]string {
	seen := make(map[string]map[string]bool)
	for _, item := range recent {
		if !activeSessions[item.SessionID] {
			continue
		}
		path := normalizeWorkspacePath(workspaceRoot, item.Path)
		if path == "" {
			continue
		}
		if seen[item.SessionID] == nil {
			seen[item.SessionID] = make(map[string]bool)
		}
		seen[item.SessionID][path] = true
	}
	result := make(map[string][]string, len(seen))
	for sessionID, files := range seen {
		for file := range files {
			result[sessionID] = append(result[sessionID], file)
		}
		sort.Strings(result[sessionID])
	}
	return result
}

func activeSessionIDs(sessions []session.Session) map[string]bool {
	ids := make(map[string]bool, len(sessions))
	for _, item := range sessions {
		ids[item.ID] = true
	}
	return ids
}

func uniqueFiles(sessionFiles map[string][]string) []string {
	seen := make(map[string]bool)
	for _, files := range sessionFiles {
		for _, file := range files {
			seen[file] = true
		}
	}
	result := make([]string, 0, len(seen))
	for file := range seen {
		result = append(result, file)
	}
	sort.Strings(result)
	return result
}

func normalizeWorkspacePath(workspaceRoot, path string) string {
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		if rel, err := filepath.Rel(workspaceRoot, path); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(path)
}

func matchingImpacts(sourceSession, dependentSession, changedFile string, symbols []store.AnalysisSymbol, edges []store.DependencyEdge) []semanticImpact {
	var impacts []semanticImpact
	symbolNames := make(map[string]bool)
	for _, symbol := range symbols {
		symbolNames[symbol.Name] = true
	}
	for _, edge := range edges {
		if dependencyTargetsFile(edge.Target, changedFile) {
			impacts = append(impacts, semanticImpact{SourceSession: sourceSession, DependentSession: dependentSession, ChangedFile: changedFile, DependentFile: edge.SourceFile, Symbol: firstSymbolName(symbols), Dependency: edge.Target, Confidence: edge.Confidence})
			continue
		}
		if edge.TargetSymbol != "" && symbolNames[edge.TargetSymbol] {
			impacts = append(impacts, semanticImpact{SourceSession: sourceSession, DependentSession: dependentSession, ChangedFile: changedFile, DependentFile: edge.SourceFile, Symbol: edge.TargetSymbol, Dependency: edge.Target, Confidence: edge.Confidence})
		}
	}
	return impacts
}

func dependencyTargetsFile(target, changedFile string) bool {
	if target == "" || changedFile == "" {
		return false
	}
	candidates := dependencyCandidates(changedFile)
	for _, candidate := range candidates {
		if target == candidate || strings.HasSuffix(target, "/"+candidate) || strings.HasSuffix(candidate, "/"+target) {
			return true
		}
	}
	return false
}

func dependencyCandidates(file string) []string {
	withoutExt := strings.TrimSuffix(file, filepath.Ext(file))
	dir := filepath.Dir(file)
	base := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	values := []string{file, withoutExt, dir, base}
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.Trim(value, ".")
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, filepath.ToSlash(value))
	}
	return result
}

func firstSymbolName(symbols []store.AnalysisSymbol) string {
	if len(symbols) == 0 {
		return ""
	}
	return symbols[0].Name
}

func sessionNameMap(sessions []session.Session) map[string]string {
	nameMap := make(map[string]string, len(sessions))
	for _, s := range sessions {
		if s.Name != "" {
			nameMap[s.ID] = s.Name
		}
	}
	return nameMap
}

func (d *Daemon) boardActivityStats(ctx context.Context, sessions []session.Session) (map[string]board.ActivityStats, error) {
	stats := make(map[string]board.ActivityStats, len(sessions))
	for _, item := range sessions {
		recent, err := d.activityStore.RecentBySession(ctx, item.ID, time.Now().Add(-activity.CompressedWindow).Unix(), 100)
		if err != nil {
			return nil, err
		}
		digest := activity.DecayActivities(recent, time.Now(), 3, 3)
		stats[item.ID] = board.ActivityStats{
			RecentFiles:       digest.FullFiles,
			HotDirectories:    activity.HotDirectories(digest.FullFiles, 3),
			ActivitySummaries: digest.Lines(),
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
	item := store.AgentState{WorkspaceRoot: stateWorkspaceRoot(request), Key: key, ValueJSON: valueJSON, UpdatedAt: time.Now().Unix(), SessionID: current.ID}
	if err := d.stateStore.Set(context.Background(), item); err != nil {
		return protocol.Failure(err.Error())
	}
	return protocol.Success(map[string]any{"key": key})
}

func (d *Daemon) handleStateGet(request protocol.Request) protocol.Response {
	workspaceRoot := stateWorkspaceRoot(request)
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
	workspaceRoot := stateWorkspaceRoot(request)
	items, err := d.stateStore.List(context.Background(), workspaceRoot, payloadString(request, "prefix"))
	if err != nil {
		return protocol.Failure(err.Error())
	}
	return protocol.Success(map[string]any{"items": stateItemsPayload(items)})
}

func (d *Daemon) handleStateDelete(request protocol.Request) protocol.Response {
	workspaceRoot := stateWorkspaceRoot(request)
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

func stateWorkspaceRoot(request protocol.Request) string {
	if payloadString(request, "scope") == "global" {
		return store.GlobalWorkspaceRoot
	}
	return payloadString(request, "workspace_root")
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
	if current.RepoID != "" {
		repoSessions, repoErr := d.manager.ListActiveByRepo(context.Background(), current.RepoID)
		if repoErr == nil {
			sessions = repoSessions
		}
	}
	if err != nil {
		return protocol.Failure(err.Error())
	}
	result := gitguard.Preflight(gitguard.PreflightInput{
		Intent:           intent,
		CurrentSession:   current,
		ActiveSessions:   sessions,
		FileOverlaps:     d.gitPreflightOverlaps(context.Background(), current.WorkspaceRoot, current.ID),
		SemanticOverlaps: d.gitPreflightSemanticOverlaps(context.Background(), current.WorkspaceRoot, current.ID, sessions),
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

func (d *Daemon) historyActivitySummaries(ctx context.Context, items []session.Session) map[string][]string {
	if d.activityStore == (store.FileActivityStore{}) {
		return nil
	}
	result := make(map[string][]string, len(items))
	for _, item := range items {
		recent, err := d.activityStore.RecentBySession(ctx, item.ID, time.Now().Add(-activity.CompressedWindow).Unix(), 100)
		if err != nil {
			continue
		}
		digest := activity.DecayActivities(recent, time.Now(), 3, 3)
		lines := digest.Lines()
		if len(digest.FullFiles) > 0 {
			lines = append([]string{"recent files: " + strings.Join(digest.FullFiles, ", ")}, lines...)
		}
		if len(lines) > 0 {
			result[item.ID] = lines
		}
	}
	return result
}

type workLogStats struct {
	Files     int
	GitEvents int
}

func (d *Daemon) renderWorkLog(ctx context.Context, workspaceRoot string, items []session.Session, since int64, now time.Time) string {
	stats := d.workLogStats(ctx, workspaceRoot, items, since)
	var out strings.Builder
	fmt.Fprintf(&out, "[Pane] Work log\n")
	fmt.Fprintf(&out, "Workspace: %s\n", workspaceRoot)
	if since > 0 {
		fmt.Fprintf(&out, "Since: %s\n", time.Unix(since, 0).Format(time.RFC3339))
	}
	if len(items) == 0 {
		fmt.Fprintf(&out, "No sessions recorded.\n")
		return out.String()
	}
	var totalFiles, totalGit int
	for _, item := range items {
		itemStats := stats[item.ID]
		totalFiles += itemStats.Files
		totalGit += itemStats.GitEvents
	}
	fmt.Fprintf(&out, "Sessions: %d\n", len(items))
	fmt.Fprintf(&out, "Files touched: %d\n", totalFiles)
	fmt.Fprintf(&out, "Git operations: %d\n", totalGit)
	for _, item := range items {
		itemStats := stats[item.ID]
		fmt.Fprintf(&out, "\n- %s — %s", historySessionLabel(item), statusLabel(item.Status))
		if item.Branch != "" {
			fmt.Fprintf(&out, " — %s", item.Branch)
		}
		fmt.Fprintf(&out, "\n")
		fmt.Fprintf(&out, "  Intent: %s\n", displayText(item.LastIntent, "not set"))
		fmt.Fprintf(&out, "  Duration: %s\n", sessionDuration(item, now))
		fmt.Fprintf(&out, "  Files touched: %d\n", itemStats.Files)
		fmt.Fprintf(&out, "  Git operations: %d\n", itemStats.GitEvents)
	}
	return out.String()
}

func (d *Daemon) workLogStats(ctx context.Context, workspaceRoot string, items []session.Session, since int64) map[string]workLogStats {
	stats := make(map[string]workLogStats, len(items))
	if since <= 0 {
		since = 0
	}
	for _, item := range items {
		var itemStats workLogStats
		if d.activityStore != (store.FileActivityStore{}) {
			activities, err := d.activityStore.RecentBySession(ctx, item.ID, since, 10000)
			if err == nil {
				files := make(map[string]bool)
				for _, activity := range activities {
					files[activity.Path] = true
				}
				itemStats.Files = len(files)
			}
		}
		stats[item.ID] = itemStats
	}
	if d.gitEventStore != (store.GitEventStore{}) {
		events, err := d.gitEventStore.RecentByWorkspace(ctx, workspaceRoot, since, 10000)
		if err == nil {
			for _, event := range events {
				itemStats := stats[event.SessionID]
				itemStats.GitEvents++
				stats[event.SessionID] = itemStats
			}
		}
	}
	return stats
}

func sessionDuration(item session.Session, now time.Time) string {
	end := item.LastSeenAt
	if item.Status != session.StatusClosed {
		end = now.Unix()
	}
	if item.StartedAt <= 0 || end <= item.StartedAt {
		return "unknown"
	}
	return (time.Duration(end-item.StartedAt) * time.Second).Round(time.Second).String()
}

func renderHistory(workspaceRoot string, items []session.Session, activitySummaries map[string][]string, showLineage bool, now time.Time) string {
	var out strings.Builder
	fmt.Fprintf(&out, "[Pane] Session history\n")
	fmt.Fprintf(&out, "Workspace: %s\n", workspaceRoot)
	if len(items) == 0 {
		fmt.Fprintf(&out, "No sessions recorded.\n")
		return out.String()
	}
	if showLineage {
		fmt.Fprintf(&out, "\nLineage tree:\n")
		renderLineageTree(&out, items)
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
			fmt.Fprintf(&out, " — child of %s", session.ShortID(item.ParentID))
		}
		fmt.Fprintf(&out, "\n  Intent: %s\n", displayText(item.LastIntent, "not set"))
		if chain := lineageChain(items, item.ID); len(chain) > 1 {
			fmt.Fprintf(&out, "  Lineage: %s\n", strings.Join(chain, " > "))
		}
		if children := childSessionLabels(items, item.ID); len(children) > 0 {
			fmt.Fprintf(&out, "  Children: %s\n", strings.Join(children, ", "))
		}
		fmt.Fprintf(&out, "  CWD: %s\n", displayText(item.CWD, "unknown"))
		fmt.Fprintf(&out, "  Last seen: %s\n", relativeTime(item.LastSeenAt, now))
		if summaries := activitySummaries[item.ID]; len(summaries) > 0 {
			fmt.Fprintf(&out, "  Activity summary: %s\n", strings.Join(summaries, "; "))
		}
	}
	return out.String()
}

func renderLineageTree(out *strings.Builder, items []session.Session) {
	children := childrenByParent(items)
	itemByID := sessionsByID(items)
	for _, item := range items {
		if _, parentVisible := itemByID[item.ParentID]; item.ParentID != "" && parentVisible {
			continue
		}
		renderLineageNode(out, item, children, 0)
	}
}

func renderLineageNode(out *strings.Builder, item session.Session, children map[string][]session.Session, depth int) {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(out, "  %s- %s", indent, historySessionLabel(item))
	if item.LastIntent != "" {
		fmt.Fprintf(out, ": %s", item.LastIntent)
	}
	fmt.Fprintf(out, "\n")
	for _, child := range children[item.ID] {
		renderLineageNode(out, child, children, depth+1)
	}
}

func childSessionLabels(items []session.Session, parentID string) []string {
	children := childrenByParent(items)[parentID]
	labels := make([]string, 0, len(children))
	for _, child := range children {
		labels = append(labels, historySessionLabel(child))
	}
	return labels
}

func lineageChain(items []session.Session, sessionID string) []string {
	itemByID := sessionsByID(items)
	chain := make([]string, 0)
	seen := make(map[string]bool)
	current, ok := itemByID[sessionID]
	for ok && !seen[current.ID] {
		seen[current.ID] = true
		chain = append([]string{historySessionLabel(current)}, chain...)
		if current.ParentID == "" {
			break
		}
		current, ok = itemByID[current.ParentID]
	}
	return chain
}

func childrenByParent(items []session.Session) map[string][]session.Session {
	children := make(map[string][]session.Session)
	for _, item := range items {
		if item.ParentID != "" {
			children[item.ParentID] = append(children[item.ParentID], item)
		}
	}
	return children
}

func sessionsByID(items []session.Session) map[string]session.Session {
	byID := make(map[string]session.Session, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	return byID
}

func historySessionLabel(item session.Session) string {
	if item.Name != "" {
		return item.Name
	}
	return session.ShortID(item.ID)
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
		"repo_id":           value.RepoID,
		"git_common_dir":    value.GitCommonDir,
		"intent":            value.LastIntent,
		"started_at":        value.StartedAt,
		"last_seen_at":      value.LastSeenAt,
		"status":            string(value.Status),
		"parent_session_id": value.ParentID,
	}
}
