package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestShellInitOutput(t *testing.T) {
	var stdout bytes.Buffer
	if err := runShellInit(nil, &stdout); err != nil {
		t.Fatalf("runShellInit returned error: %v", err)
	}
	got := stdout.String()
	for _, want := range []string{"PANE_BIN=", "_pane_start_daemon", "_pane_heartbeat", "daemon start", "summary"} {
		if !strings.Contains(got, want) {
			t.Fatalf("shell init missing %q:\n%s", want, got)
		}
	}
}
