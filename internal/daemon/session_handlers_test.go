package daemon

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/juliancanalez/pane/internal/activity"
	"github.com/juliancanalez/pane/internal/protocol"
	"github.com/juliancanalez/pane/internal/session"
	"github.com/juliancanalez/pane/internal/store"
)

func TestSessionAndBoardHandlers(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	d := NewForTest(Config{SocketPath: "test.sock"}, session.NewManager(store.NewSessionStore(db)), store.NewMessageStore(db))
	d.activityStore = store.NewFileActivityStore(db)
	env := map[string]any{
		"pane_id":        "tty:/dev/ttys001",
		"tty":            "/dev/ttys001",
		"workspace_root": "/workspace",
		"cwd":            "/workspace/src",
		"branch":         "main",
	}

	initResponse := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: env}, func() {})
	if !initResponse.OK {
		t.Fatalf("init failed: %#v", initResponse)
	}
	if initResponse.Payload["session_id"] == "" {
		t.Fatalf("missing session id: %#v", initResponse.Payload)
	}
	sessionID := initResponse.Payload["session_id"].(string)

	intentPayload := map[string]any{
		"pane_id":        "tty:/dev/ttys001",
		"workspace_root": "/workspace",
		"intent":         "testing daemon-backed board",
	}
	intentResponse := d.Handle(protocol.Request{Type: protocol.RequestSessionIntent, Payload: intentPayload}, func() {})
	if !intentResponse.OK {
		t.Fatalf("intent failed: %#v", intentResponse)
	}

	heartbeatPayload := map[string]any{
		"pane_id":        "tty:/dev/ttys001",
		"tty":            "/dev/ttys001",
		"workspace_root": "/workspace",
		"cwd":            "/workspace/other",
		"branch":         "feature",
	}
	heartbeatResponse := d.Handle(protocol.Request{Type: protocol.RequestSessionHeartbeat, Payload: heartbeatPayload}, func() {})
	if !heartbeatResponse.OK {
		t.Fatalf("heartbeat failed: %#v", heartbeatResponse)
	}
	if heartbeatResponse.Payload["intent"] != "testing daemon-backed board" || heartbeatResponse.Payload["cwd"] != "/workspace/other" {
		t.Fatalf("unexpected heartbeat payload: %#v", heartbeatResponse.Payload)
	}

	statusResponse := d.Handle(protocol.Request{Type: protocol.RequestSessionStatus, Payload: env}, func() {})
	if !statusResponse.OK {
		t.Fatalf("status failed: %#v", statusResponse)
	}
	if statusResponse.Payload["intent"] != "testing daemon-backed board" {
		t.Fatalf("unexpected status payload: %#v", statusResponse.Payload)
	}

	boardResponse := d.Handle(protocol.Request{Type: protocol.RequestGetBoard, Payload: map[string]any{"workspace_root": "/workspace"}}, func() {})
	if !boardResponse.OK {
		t.Fatalf("board failed: %#v", boardResponse)
	}
	text, ok := boardResponse.Payload["text"].(string)
	if !ok || !strings.Contains(text, "short: "+session.ShortID(sessionID)) || !strings.Contains(text, "testing daemon-backed board") {
		t.Fatalf("unexpected board text: %#v", boardResponse.Payload)
	}

	summaryResponse := d.Handle(protocol.Request{Type: protocol.RequestGetSummary, Payload: env}, func() {})
	if !summaryResponse.OK {
		t.Fatalf("summary failed: %#v", summaryResponse)
	}
	summaryText, ok := summaryResponse.Payload["text"].(string)
	if !ok || !strings.Contains(summaryText, "[Pane] Session summary") || !strings.Contains(summaryText, "testing daemon-backed board") {
		t.Fatalf("unexpected summary text: %#v", summaryResponse.Payload)
	}

	closeResponse := d.Handle(protocol.Request{Type: protocol.RequestSessionClose, Payload: env}, func() {})
	if !closeResponse.OK || closeResponse.Payload["status"] != "closed" {
		t.Fatalf("close failed: %#v", closeResponse)
	}
	boardAfterClose := d.Handle(protocol.Request{Type: protocol.RequestGetBoard, Payload: map[string]any{"workspace_root": "/workspace"}}, func() {})
	if !boardAfterClose.OK || strings.Contains(boardAfterClose.Payload["text"].(string), sessionID) {
		t.Fatalf("closed session should not appear on board: %#v", boardAfterClose)
	}
}

func TestStateGlobalScopeAndSummaryState(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	d := NewForTest(Config{SocketPath: "test.sock"}, session.NewManager(store.NewSessionStore(db)), store.NewMessageStore(db))
	d.stateStore = store.NewAgentStateStore(db)
	d.activityStore = store.NewFileActivityStore(db)
	env := map[string]any{"pane_id": "pane-a", "workspace_root": "/workspace", "cwd": "/workspace", "branch": "main"}
	init := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: env}, func() {})
	if !init.OK {
		t.Fatalf("init failed: %#v", init)
	}

	setSummary := d.Handle(protocol.Request{Type: protocol.RequestStateSet, Payload: map[string]any{"pane_id": "pane-a", "workspace_root": "/workspace", "key": "summary.note", "value_json": `{"text":"watch auth"}`}}, func() {})
	if !setSummary.OK {
		t.Fatalf("summary state set failed: %#v", setSummary)
	}
	summaryResponse := d.Handle(protocol.Request{Type: protocol.RequestGetSummary, Payload: env}, func() {})
	if !summaryResponse.OK || !strings.Contains(summaryResponse.Payload["text"].(string), "Shared state:") || !strings.Contains(summaryResponse.Payload["text"].(string), "summary.note") {
		t.Fatalf("summary should include summary.* state: %#v", summaryResponse)
	}

	setGlobal := d.Handle(protocol.Request{Type: protocol.RequestStateSet, Payload: map[string]any{"pane_id": "pane-a", "workspace_root": "/workspace", "scope": "global", "key": "agent.memory", "value_json": `{"status":"global"}`}}, func() {})
	if !setGlobal.OK {
		t.Fatalf("global state set failed: %#v", setGlobal)
	}
	getGlobal := d.Handle(protocol.Request{Type: protocol.RequestStateGet, Payload: map[string]any{"workspace_root": "/other", "scope": "global", "key": "agent.memory"}}, func() {})
	if !getGlobal.OK || getGlobal.Payload["value_json"] != `{"status":"global"}` {
		t.Fatalf("global state get failed: %#v", getGlobal)
	}
}

func TestRenderWorkLog(t *testing.T) {
	d := &Daemon{}
	items := []session.Session{{ID: "session-a", Status: session.StatusClosed, Branch: "main", LastIntent: "ship thing", StartedAt: 100, LastSeenAt: 160}}

	got := d.renderWorkLog(context.Background(), "/workspace", items, 0, time.Unix(200, 0))
	for _, want := range []string{"[Pane] Work log", "Sessions: 1", "ship thing", "Duration: 1m0s", "Files touched:", "Git operations:"} {
		if !strings.Contains(got, want) {
			t.Fatalf("work log missing %q:\n%s", want, got)
		}
	}
}

func TestRenderHistoryShowsLineageChains(t *testing.T) {
	items := []session.Session{
		{ID: "session-root", Name: "root", WorkspaceRoot: "/workspace", CWD: "/workspace", Status: session.StatusClosed, LastIntent: "root work", LastSeenAt: 100},
		{ID: "session-child", WorkspaceRoot: "/workspace", CWD: "/workspace", Status: session.StatusClosed, ParentID: "session-root", LastIntent: "child work", LastSeenAt: 110},
		{ID: "session-grandchild", WorkspaceRoot: "/workspace", CWD: "/workspace", Status: session.StatusActive, ParentID: "session-child", LastIntent: "grandchild work", LastSeenAt: 120},
	}

	got := renderHistory("/workspace", items, nil, true, time.Unix(130, 0))
	for _, want := range []string{
		"Lineage tree:",
		"- root: root work",
		"- child: child work",
		"- grandch", // shortened grandchild id
		"Lineage: root > child > grandch",
		"Children: child",
		"child of child",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("history missing %q:\n%s", want, got)
		}
	}
}

func TestRecordWatchEventAttributesSharedCWDToMultipleSessions(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	d := NewForTest(Config{SocketPath: "test.sock"}, session.NewManager(store.NewSessionStore(db)), store.NewMessageStore(db))
	d.activityStore = store.NewFileActivityStore(db)
	envA := map[string]any{"pane_id": "pane-a", "workspace_root": "/workspace", "cwd": "/workspace", "branch": "main"}
	envB := map[string]any{"pane_id": "pane-b", "workspace_root": "/workspace", "cwd": "/workspace", "branch": "main"}
	initA := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: envA}, func() {})
	initB := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: envB}, func() {})
	if !initA.OK || !initB.OK {
		t.Fatalf("init failed: %#v %#v", initA, initB)
	}

	d.recordWatchEvent("/workspace", activity.WatchEvent{Path: "/workspace/shared.txt", EventType: activity.EventModified, Time: time.Now()})

	pathSessions, err := d.activityStore.OverlapByWorkspace(context.Background(), "/workspace", time.Now().Add(-time.Minute).Unix())
	if err != nil {
		t.Fatalf("OverlapByWorkspace returned error: %v", err)
	}
	sessions := pathSessions["shared.txt"]
	if len(sessions) != 2 {
		t.Fatalf("shared.txt sessions = %#v", sessions)
	}
}

func TestAttributeSessionsPrefersMostSpecificCWD(t *testing.T) {
	owners := attributeSessions([]session.Session{
		{ID: "root", CWD: "/workspace"},
		{ID: "nested", CWD: "/workspace/pkg"},
	}, "/workspace/pkg/file.go")
	if len(owners) != 1 || owners[0].Session.ID != "nested" || owners[0].Attribution != activity.AttributionMedium {
		t.Fatalf("owners = %#v", owners)
	}
}

func TestBoardRepoScopeShowsSiblingWorktrees(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	d := NewForTest(Config{SocketPath: "test.sock"}, session.NewManager(store.NewSessionStore(db)), store.NewMessageStore(db))
	d.activityStore = store.NewFileActivityStore(db)
	envA := map[string]any{"pane_id": "pane-a", "workspace_root": "/repo-main", "cwd": "/repo-main", "branch": "main", "repo_id": "/repo/.git", "git_common_dir": "/repo/.git"}
	envB := map[string]any{"pane_id": "pane-b", "workspace_root": "/repo-feature", "cwd": "/repo-feature", "branch": "feature", "repo_id": "/repo/.git", "git_common_dir": "/repo/.git"}
	if initA := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: envA}, func() {}); !initA.OK {
		t.Fatalf("init A failed: %#v", initA)
	}
	if initB := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: envB}, func() {}); !initB.OK {
		t.Fatalf("init B failed: %#v", initB)
	}

	workspaceBoard := d.Handle(protocol.Request{Type: protocol.RequestGetBoard, Payload: map[string]any{"workspace_root": "/repo-main", "repo_id": "/repo/.git"}}, func() {})
	if !workspaceBoard.OK {
		t.Fatalf("workspace board failed: %#v", workspaceBoard)
	}
	if !strings.Contains(workspaceBoard.Payload["text"].(string), "Sessions: 1") {
		t.Fatalf("workspace board should stay worktree-local:\n%s", workspaceBoard.Payload["text"].(string))
	}

	repoBoard := d.Handle(protocol.Request{Type: protocol.RequestGetBoard, Payload: map[string]any{"workspace_root": "/repo-main", "repo_id": "/repo/.git", "scope": "repo"}}, func() {})
	if !repoBoard.OK {
		t.Fatalf("repo board failed: %#v", repoBoard)
	}
	text := repoBoard.Payload["text"].(string)
	for _, want := range []string{"Scope: repository", "Sessions: 2", "Worktree: /repo-feature"} {
		if !strings.Contains(text, want) {
			t.Fatalf("repo board missing %q:\n%s", want, text)
		}
	}
}

func TestBoardMachineScopeAndCrossWorkspaceSend(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	d := NewForTest(Config{SocketPath: "test.sock"}, session.NewManager(store.NewSessionStore(db)), store.NewMessageStore(db))
	d.activityStore = store.NewFileActivityStore(db)
	// Two sessions in entirely separate repositories/workspaces.
	envA := map[string]any{"pane_id": "pane-a", "workspace_root": "/repo-one", "cwd": "/repo-one", "branch": "main", "repo_id": "/repo-one/.git", "git_common_dir": "/repo-one/.git"}
	envB := map[string]any{"pane_id": "pane-b", "workspace_root": "/repo-two", "cwd": "/repo-two", "branch": "main", "repo_id": "/repo-two/.git", "git_common_dir": "/repo-two/.git"}
	initA := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: envA}, func() {})
	initB := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: envB}, func() {})
	if !initA.OK || !initB.OK {
		t.Fatalf("init failed: %#v %#v", initA, initB)
	}
	sessionB := initB.Payload["session_id"].(string)

	// Default workspace board for A sees only A.
	local := d.Handle(protocol.Request{Type: protocol.RequestGetBoard, Payload: map[string]any{"workspace_root": "/repo-one", "repo_id": "/repo-one/.git"}}, func() {})
	if !strings.Contains(local.Payload["text"].(string), "Sessions: 1") {
		t.Fatalf("workspace board should be local-only:\n%s", local.Payload["text"].(string))
	}

	// Machine board sees both sessions across repos, with each workspace shown.
	machine := d.Handle(protocol.Request{Type: protocol.RequestGetBoard, Payload: map[string]any{"workspace_root": "/repo-one", "scope": "machine"}}, func() {})
	if !machine.OK {
		t.Fatalf("machine board failed: %#v", machine)
	}
	text := machine.Payload["text"].(string)
	for _, want := range []string{"Machine board", "Scope: machine", "Sessions: 2", "Workspace: /repo-two"} {
		if !strings.Contains(text, want) {
			t.Fatalf("machine board missing %q:\n%s", want, text)
		}
	}

	// A workspace-scoped send to the foreign session is refused.
	refused := d.Handle(protocol.Request{Type: protocol.RequestMessageSend, Payload: map[string]any{"pane_id": "pane-a", "workspace_root": "/repo-one", "to_session": sessionB, "body": "hi"}}, func() {})
	if refused.OK {
		t.Fatalf("workspace-scoped send to foreign session should fail, got %#v", refused)
	}

	// A global send to the foreign session's full ID succeeds and delivers.
	sent := d.Handle(protocol.Request{Type: protocol.RequestMessageSend, Payload: map[string]any{"pane_id": "pane-a", "workspace_root": "/repo-one", "to_session": sessionB, "body": "ping from repo-one", "scope": "global"}}, func() {})
	if !sent.OK {
		t.Fatalf("global send failed: %#v", sent)
	}
	if sent.Payload["to_session"].(string) != sessionB {
		t.Fatalf("global send to_session = %v, want %s", sent.Payload["to_session"], sessionB)
	}

	// B's inbox (in its own workspace) receives it.
	inbox := d.Handle(protocol.Request{Type: protocol.RequestMessageList, Payload: map[string]any{"pane_id": "pane-b", "workspace_root": "/repo-two"}}, func() {})
	if !inbox.OK {
		t.Fatalf("inbox failed: %#v", inbox)
	}
	if !strings.Contains(inbox.Payload["text"].(string), "ping from repo-one") {
		t.Fatalf("foreign message not delivered to inbox:\n%s", inbox.Payload["text"].(string))
	}
}

func TestGitPreflightWarnsAcrossSiblingWorktrees(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	d := NewForTest(Config{SocketPath: "test.sock"}, session.NewManager(store.NewSessionStore(db)), store.NewMessageStore(db))
	envA := map[string]any{"pane_id": "pane-a", "workspace_root": "/repo-main", "cwd": "/repo-main", "branch": "main", "repo_id": "/repo/.git", "git_common_dir": "/repo/.git"}
	envB := map[string]any{"pane_id": "pane-b", "workspace_root": "/repo-feature", "cwd": "/repo-feature", "branch": "main", "repo_id": "/repo/.git", "git_common_dir": "/repo/.git"}
	if initA := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: envA}, func() {}); !initA.OK {
		t.Fatalf("init A failed: %#v", initA)
	}
	if initB := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: envB}, func() {}); !initB.OK {
		t.Fatalf("init B failed: %#v", initB)
	}

	preflight := d.Handle(protocol.Request{Type: protocol.RequestGitPreflight, Payload: map[string]any{"pane_id": "pane-a", "workspace_root": "/repo-main", "args": []string{"rebase", "main"}}}, func() {})
	if !preflight.OK {
		t.Fatalf("preflight failed: %#v", preflight)
	}
	if len(preflight.Warnings) == 0 || !strings.Contains(strings.Join(preflight.Warnings, "\n"), "also active on branch main") {
		t.Fatalf("expected same-repo branch warning, got %#v", preflight.Warnings)
	}
}

func TestBoardAndSummaryShowOverlap(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	d := NewForTest(Config{SocketPath: "test.sock"}, session.NewManager(store.NewSessionStore(db)), store.NewMessageStore(db))
	d.activityStore = store.NewFileActivityStore(db)
	envA := map[string]any{"pane_id": "pane-a", "workspace_root": "/workspace", "cwd": "/workspace", "branch": "main"}
	envB := map[string]any{"pane_id": "pane-b", "workspace_root": "/workspace", "cwd": "/workspace", "branch": "main"}
	initA := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: envA}, func() {})
	initB := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: envB}, func() {})
	if !initA.OK || !initB.OK {
		t.Fatalf("init failed: %#v %#v", initA, initB)
	}
	sessionA := initA.Payload["session_id"].(string)
	sessionB := initB.Payload["session_id"].(string)

	// Create overlapping file activity
	now := time.Now().Unix()
	for _, fa := range []struct {
		sid, path string
	}{
		{sessionA, "/workspace/shared.go"},
		{sessionB, "/workspace/shared.go"},
		{sessionA, "/workspace/solo.go"},
	} {
		if err := d.activityStore.Save(context.Background(), activity.FileActivity{
			SessionID: fa.sid, Path: fa.path, EventType: activity.EventModified,
			Attribution: activity.AttributionHigh, Timestamp: now,
		}); err != nil {
			t.Fatalf("Save activity failed: %v", err)
		}
	}

	// Board should show overlap
	boardResp := d.Handle(protocol.Request{Type: protocol.RequestGetBoard, Payload: map[string]any{"workspace_root": "/workspace"}}, func() {})
	if !boardResp.OK {
		t.Fatalf("board failed: %#v", boardResp)
	}
	boardText := boardResp.Payload["text"].(string)
	if !strings.Contains(boardText, "Overlap:") || !strings.Contains(boardText, "shared.go") {
		t.Fatalf("expected overlap in board output:\n%s", boardText)
	}

	// Summary should show overlap for session A
	summaryResp := d.Handle(protocol.Request{Type: protocol.RequestGetSummary, Payload: envA}, func() {})
	if !summaryResp.OK {
		t.Fatalf("summary failed: %#v", summaryResp)
	}
	summaryText := summaryResp.Payload["text"].(string)
	if !strings.Contains(summaryText, "Overlap:") || !strings.Contains(summaryText, "shared.go") {
		t.Fatalf("expected overlap in summary output:\n%s", summaryText)
	}
	// Overlap section should only contain shared.go, not solo.go
	overlapIdx := strings.Index(summaryText, "Overlap:")
	overlapSection := summaryText[overlapIdx:]
	// Overlap section ends at next major section ("Other sessions:")
	if endIdx := strings.Index(overlapSection, "Other sessions:"); endIdx > 0 {
		overlapSection = overlapSection[:endIdx]
	}
	if strings.Contains(overlapSection, "solo.go") {
		t.Fatalf("solo.go should not appear in overlap section:\n%s", overlapSection)
	}
}

func TestBoardSummaryAndPreflightShowSemanticOverlap(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	d := NewForTest(Config{SocketPath: "test.sock"}, session.NewManager(store.NewSessionStore(db)), store.NewMessageStore(db))
	d.activityStore = store.NewFileActivityStore(db)
	d.analysisStore = store.NewAnalysisStore(db)
	envA := map[string]any{"pane_id": "pane-a", "workspace_root": "/workspace", "cwd": "/workspace", "branch": "main"}
	envB := map[string]any{"pane_id": "pane-b", "workspace_root": "/workspace", "cwd": "/workspace", "branch": "main"}
	initA := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: envA}, func() {})
	initB := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: envB}, func() {})
	if !initA.OK || !initB.OK {
		t.Fatalf("init failed: %#v %#v", initA, initB)
	}
	sessionA := initA.Payload["session_id"].(string)
	sessionB := initB.Payload["session_id"].(string)

	if err := d.analysisStore.UpsertFile(context.Background(), store.FileAnalysis{
		WorkspaceRoot: "/workspace",
		File:          "crypto/token.go",
		Language:      "go",
		Symbols:       []store.AnalysisSymbol{{Name: "ValidateToken", Kind: "function", StartLine: 3, EndLine: 7}},
	}); err != nil {
		t.Fatalf("upsert changed file analysis: %v", err)
	}
	if err := d.analysisStore.UpsertFile(context.Background(), store.FileAnalysis{
		WorkspaceRoot: "/workspace",
		File:          "auth/handler.go",
		Language:      "go",
		Dependencies:  []store.DependencyEdge{{Target: "github.com/example/project/crypto", Kind: "import", Confidence: 0.9}},
	}); err != nil {
		t.Fatalf("upsert dependent file analysis: %v", err)
	}

	now := time.Now().Unix()
	for _, fa := range []struct {
		sid, path string
	}{
		{sessionA, "/workspace/crypto/token.go"},
		{sessionB, "/workspace/auth/handler.go"},
	} {
		if err := d.activityStore.Save(context.Background(), activity.FileActivity{SessionID: fa.sid, Path: fa.path, EventType: activity.EventModified, Attribution: activity.AttributionHigh, Timestamp: now}); err != nil {
			t.Fatalf("save activity: %v", err)
		}
	}

	boardResp := d.Handle(protocol.Request{Type: protocol.RequestGetBoard, Payload: map[string]any{"workspace_root": "/workspace"}}, func() {})
	if !boardResp.OK {
		t.Fatalf("board failed: %#v", boardResp)
	}
	boardText := boardResp.Payload["text"].(string)
	if !strings.Contains(boardText, "Semantic overlap:") || !strings.Contains(boardText, "ValidateToken") || !strings.Contains(boardText, "auth/handler.go") {
		t.Fatalf("expected semantic overlap in board output:\n%s", boardText)
	}

	summaryResp := d.Handle(protocol.Request{Type: protocol.RequestGetSummary, Payload: envA}, func() {})
	if !summaryResp.OK {
		t.Fatalf("summary failed: %#v", summaryResp)
	}
	summaryText := summaryResp.Payload["text"].(string)
	if !strings.Contains(summaryText, "Semantic overlap:") || !strings.Contains(summaryText, "ValidateToken") {
		t.Fatalf("expected semantic overlap in summary output:\n%s", summaryText)
	}

	preflightResp := d.Handle(protocol.Request{Type: protocol.RequestGitPreflight, Payload: map[string]any{"pane_id": "pane-a", "workspace_root": "/workspace", "args": []string{"merge", "main"}}}, func() {})
	if !preflightResp.OK {
		t.Fatalf("preflight failed: %#v", preflightResp)
	}
	joinedWarnings := strings.Join(preflightResp.Warnings, "\n")
	if !strings.Contains(joinedWarnings, "depends on ValidateToken") {
		t.Fatalf("expected semantic preflight warning, got %#v", preflightResp.Warnings)
	}
}

func TestMessageHandlers(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	d := NewForTest(Config{SocketPath: "test.sock"}, session.NewManager(store.NewSessionStore(db)), store.NewMessageStore(db))
	d.activityStore = store.NewFileActivityStore(db)
	envA := map[string]any{"pane_id": "pane-a", "workspace_root": "/workspace", "cwd": "/workspace", "branch": "main"}
	envB := map[string]any{"pane_id": "pane-b", "workspace_root": "/workspace", "cwd": "/workspace", "branch": "main"}
	initA := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: envA}, func() {})
	initB := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: envB}, func() {})
	if !initA.OK || !initB.OK {
		t.Fatalf("init failed: %#v %#v", initA, initB)
	}
	sessionA := initA.Payload["session_id"].(string)
	sessionB := initB.Payload["session_id"].(string)

	sendMissing := d.Handle(protocol.Request{Type: protocol.RequestMessageSend, Payload: map[string]any{"pane_id": "pane-a", "workspace_root": "/workspace", "to_session": "missing", "body": "Are you done?"}}, func() {})
	if sendMissing.OK || !strings.Contains(sendMissing.Error, "not found") {
		t.Fatalf("expected missing target failure: %#v", sendMissing)
	}

	send := d.Handle(protocol.Request{Type: protocol.RequestMessageSend, Payload: map[string]any{"pane_id": "pane-a", "workspace_root": "/workspace", "to_session": session.ShortID(sessionB), "body": "Are you done?"}}, func() {})
	if !send.OK {
		t.Fatalf("send failed: %#v", send)
	}
	messageID := send.Payload["message_id"].(string)

	board := d.Handle(protocol.Request{Type: protocol.RequestGetBoard, Payload: map[string]any{"workspace_root": "/workspace"}}, func() {})
	if !board.OK || !strings.Contains(board.Payload["text"].(string), "1 unread") {
		t.Fatalf("expected board unread indicator: %#v", board)
	}
	summaryB := d.Handle(protocol.Request{Type: protocol.RequestGetSummary, Payload: envB}, func() {})
	if !summaryB.OK || !strings.Contains(summaryB.Payload["text"].(string), "Unread messages: 1") {
		t.Fatalf("expected summary unread indicator: %#v", summaryB)
	}

	inboxB := d.Handle(protocol.Request{Type: protocol.RequestMessageList, Payload: envB}, func() {})
	if !inboxB.OK {
		t.Fatalf("inbox failed: %#v", inboxB)
	}
	text := inboxB.Payload["text"].(string)
	if !strings.Contains(text, messageID) || !strings.Contains(text, "Are you done?") {
		t.Fatalf("unexpected inbox: %s", text)
	}

	reply := d.Handle(protocol.Request{Type: protocol.RequestMessageReply, Payload: map[string]any{"pane_id": "pane-b", "workspace_root": "/workspace", "message_id": messageID, "body": "Yes"}}, func() {})
	if !reply.OK {
		t.Fatalf("reply failed: %#v", reply)
	}
	if reply.Payload["to_session"] != sessionA {
		t.Fatalf("reply routed to wrong session: %#v", reply.Payload)
	}

	inboxA := d.Handle(protocol.Request{Type: protocol.RequestMessageList, Payload: envA}, func() {})
	if !inboxA.OK {
		t.Fatalf("inbox A failed: %#v", inboxA)
	}
	text = inboxA.Payload["text"].(string)
	if !strings.Contains(text, "Yes") {
		t.Fatalf("unexpected reply inbox: %s", text)
	}
}
