package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallShellHookIsIdempotent(t *testing.T) {
	rcPath := filepath.Join(t.TempDir(), ".zshrc")
	paneBin := filepath.Join(t.TempDir(), "bin", "pane")
	if err := installShellHook(rcPath, paneBin); err != nil {
		t.Fatalf("installShellHook returned error: %v", err)
	}
	if err := installShellHook(rcPath, paneBin); err != nil {
		t.Fatalf("second installShellHook returned error: %v", err)
	}
	content, err := os.ReadFile(rcPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if strings.Count(string(content), shellHookMarkerStart) != 1 {
		t.Fatalf("expected one shell hook block, got:\n%s", content)
	}
}

func TestInstallGitShim(t *testing.T) {
	home := t.TempDir()
	shimPath, err := installGitShim("/tmp/pane", home)
	if err != nil {
		t.Fatalf("installGitShim returned error: %v", err)
	}
	content, err := os.ReadFile(shimPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !strings.Contains(string(content), "exec \"/tmp/pane\" git") {
		t.Fatalf("unexpected shim content: %s", content)
	}
}

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
