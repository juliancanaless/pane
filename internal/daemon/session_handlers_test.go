package daemon

import (
	"path/filepath"
	"strings"
	"testing"

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
	if !ok || !strings.Contains(text, "testing daemon-backed board") {
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

	send := d.Handle(protocol.Request{Type: protocol.RequestMessageSend, Payload: map[string]any{"pane_id": "pane-a", "workspace_root": "/workspace", "to_session": sessionB, "body": "Are you done?"}}, func() {})
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
