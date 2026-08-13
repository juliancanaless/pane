package daemon

import (
	"testing"

	"github.com/juliancanalez/pane/internal/protocol"
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
