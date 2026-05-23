package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSetupOptions(t *testing.T) {
	options, err := parseSetupOptions([]string{"--no-shell", "--no-shim", "--no-daemon"})
	if err != nil {
		t.Fatalf("parseSetupOptions returned error: %v", err)
	}
	if options.InstallShell || options.InstallShim || options.StartDaemon || options.PrintShell {
		t.Fatalf("options = %#v", options)
	}

	options, err = parseSetupOptions([]string{"--print-shell"})
	if err != nil {
		t.Fatalf("parseSetupOptions returned error: %v", err)
	}
	if !options.PrintShell || options.InstallShell {
		t.Fatalf("print-shell options = %#v", options)
	}
}

func TestFindAnalyzerSourceUsesCurrentBuildOutput(t *testing.T) {
	dir := t.TempDir()
	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd returned error: %v", err)
	}
	defer func() { _ = os.Chdir(oldwd) }()
	if err := os.MkdirAll(filepath.Join(dir, "bin"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	analyzer := filepath.Join(dir, "bin", "pane-analyze")
	if err := os.WriteFile(analyzer, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir returned error: %v", err)
	}

	got, err := findAnalyzerSource("")
	if err != nil {
		t.Fatalf("findAnalyzerSource returned error: %v", err)
	}
	gotReal, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("EvalSymlinks returned error: %v", err)
	}
	wantReal, err := filepath.EvalSymlinks(analyzer)
	if err != nil {
		t.Fatalf("EvalSymlinks returned error: %v", err)
	}
	if gotReal != wantReal {
		t.Fatalf("analyzer source = %q, want %q", gotReal, wantReal)
	}
}

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
