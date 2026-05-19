package daemon

import (
	"testing"

	"github.com/juliancanalez/pane/internal/protocol"
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
