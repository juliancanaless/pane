package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

func TestRunShutsDownOnSignal(t *testing.T) {
	dir := t.TempDir()
	config := Config{
		SocketPath: filepath.Join(dir, "pane.sock"),
		PIDPath:    filepath.Join(dir, "pane.pid"),
		LogPath:    filepath.Join(dir, "pane.log"),
	}
	stdout, stderr := os.Stdout, os.Stderr
	t.Cleanup(func() { os.Stdout, os.Stderr = stdout, stderr })

	notified := make(chan chan<- os.Signal, 1)
	originalNotify := signalNotify
	signalNotify = func(c chan<- os.Signal, _ ...os.Signal) { notified <- c }
	t.Cleanup(func() { signalNotify = originalNotify })

	done := make(chan error, 1)
	go func() { done <- New(config).Run(context.Background()) }()
	signals := <-notified
	waitHealthy(t, config.SocketPath)

	signals <- syscall.SIGTERM
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not shut down after SIGTERM")
	}

	if _, err := os.Stat(config.SocketPath); !os.IsNotExist(err) {
		t.Fatalf("socket not removed on shutdown: %v", err)
	}
	if _, err := os.Stat(config.PIDPath); !os.IsNotExist(err) {
		t.Fatalf("pid file not removed on shutdown: %v", err)
	}
	// The lock file stays on disk (removing it opens the two-daemons race,
	// see ReleaseLock); released means the flock can be taken again.
	relock, err := AcquireLock(filepath.Join(dir, "pane.lock"))
	if err != nil {
		t.Fatalf("lock not released on shutdown: %v", err)
	}
	ReleaseLock(relock)

	log, err := os.ReadFile(config.LogPath)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	if !strings.Contains(string(log), "received signal terminated; shutting down") {
		t.Fatalf("shutdown not logged:\n%s", log)
	}
	wantStart := fmt.Sprintf("daemon starting on %s (version %s, pid %d)", config.SocketPath, version.Version, os.Getpid())
	if !strings.Contains(string(log), wantStart) {
		t.Fatalf("start line = %q, want it to contain %q", log, wantStart)
	}
}

func waitHealthy(t *testing.T, socketPath string) {
	t.Helper()
	client := Client{SocketPath: socketPath, Timeout: time.Second}
	for i := 0; i < 50; i++ {
		if response, err := client.Send(protocol.Request{Type: protocol.RequestDaemonHealth}); err == nil && response.OK {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("daemon never became healthy")
}
