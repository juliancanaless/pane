package protocol

import (
	"bytes"
	"testing"
)

func TestMessageRoundTrip(t *testing.T) {
	var buffer bytes.Buffer
	original := Request{Type: RequestDaemonHealth, Payload: map[string]any{"ping": "pong"}}

	if err := WriteMessage(&buffer, original); err != nil {
		t.Fatalf("WriteMessage returned error: %v", err)
	}
	decoded, err := ReadMessage[Request](&buffer)
	if err != nil {
		t.Fatalf("ReadMessage returned error: %v", err)
	}
	if decoded.Type != original.Type {
		t.Fatalf("type = %q, want %q", decoded.Type, original.Type)
	}
	if decoded.Payload["ping"] != "pong" {
		t.Fatalf("payload = %#v", decoded.Payload)
	}
}
