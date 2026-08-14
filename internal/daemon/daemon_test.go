package daemon

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/juliancanalez/pane/internal/messages"
	"github.com/juliancanalez/pane/internal/protocol"
	"github.com/juliancanalez/pane/internal/session"
	"github.com/juliancanalez/pane/internal/store"
	"github.com/juliancanalez/pane/internal/version"
)

func TestHandleHealth(t *testing.T) {
	d := New(Config{SocketPath: "/tmp/pane-test.sock"})
	response := d.Handle(protocol.Request{Type: protocol.RequestDaemonHealth}, func() {})
	if !response.OK {
		t.Fatalf("expected OK response: %#v", response)
	}
	if response.Payload["status"] != "ok" {
		t.Fatalf("payload = %#v", response.Payload)
	}
	if response.Payload["version"] != version.Version {
		t.Fatalf("health version = %#v, want %q", response.Payload["version"], version.Version)
	}
}

func TestHandleStampsDaemonVersionOnEveryResponse(t *testing.T) {
	d := New(Config{SocketPath: "/tmp/pane-test.sock"})
	for _, requestType := range []protocol.RequestType{protocol.RequestDaemonHealth, protocol.RequestDaemonStop, protocol.RequestType("unknown")} {
		response := d.Handle(protocol.Request{Type: requestType}, func() {})
		if response.DaemonVersion != version.Version {
			t.Errorf("%s: DaemonVersion = %q, want %q", requestType, response.DaemonVersion, version.Version)
		}
	}
}

func TestHandleStop(t *testing.T) {
	d := New(Config{SocketPath: "/tmp/pane-test.sock"})
	stopped := false
	response := d.Handle(protocol.Request{Type: protocol.RequestDaemonStop}, func() { stopped = true })
	if !response.OK {
		t.Fatalf("expected OK response: %#v", response)
	}
	if !stopped {
		t.Fatal("expected stop callback")
	}
}

func TestHandleAgentContextAndMessages(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "pane.db"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()
	manager := session.NewManager(store.NewSessionStore(db))
	messageStore := store.NewMessageStore(db)
	d := NewForTest(Config{}, manager, messageStore)

	workspace := t.TempDir()
	initResponse := d.Handle(protocol.Request{Type: protocol.RequestSessionInit, Payload: map[string]any{
		"pane_id":          "pane-a",
		"workspace_root":   workspace,
		"agent_session_id": "claude-1",
	}}, func() {})
	if !initResponse.OK {
		t.Fatalf("init failed: %#v", initResponse)
	}
	sessionID, _ := initResponse.Payload["session_id"].(string)

	// Resolves with the agent session id alone — no pane/workspace identity.
	response := d.Handle(protocol.Request{Type: protocol.RequestAgentContext, Payload: map[string]any{"agent_session_id": "claude-1"}}, func() {})
	if !response.OK || response.Payload["found"] != true {
		t.Fatalf("agent context = %#v", response)
	}
	if response.Payload["session_id"] != sessionID {
		t.Fatalf("resolved %v, want %v", response.Payload["session_id"], sessionID)
	}

	unknown := d.Handle(protocol.Request{Type: protocol.RequestAgentContext, Payload: map[string]any{"agent_session_id": "nope"}}, func() {})
	if !unknown.OK || unknown.Payload["found"] != false {
		t.Fatalf("unknown agent context = %#v", unknown)
	}

	if err := messageStore.Save(context.Background(), messages.Message{
		ID: "msg-1", ThreadID: "msg-1", FromSession: sessionID, ToSession: sessionID,
		Body: "ping", State: messages.StateQueued, CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("message save: %v", err)
	}
	first := d.Handle(protocol.Request{Type: protocol.RequestAgentMessages, Payload: map[string]any{"agent_session_id": "claude-1"}}, func() {})
	if !first.OK || first.Payload["count"] != 1 {
		t.Fatalf("first agent messages = %#v", first)
	}
	second := d.Handle(protocol.Request{Type: protocol.RequestAgentMessages, Payload: map[string]any{"agent_session_id": "claude-1"}}, func() {})
	if !second.OK || second.Payload["count"] != 0 {
		t.Fatalf("delivery must mark messages read; second = %#v", second)
	}
}
