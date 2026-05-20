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

	d := NewForTest(Config{SocketPath: "test.sock"}, session.NewManager(store.NewSessionStore(db)))
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
