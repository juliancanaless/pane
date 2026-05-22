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
