package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAgentHookInputToleratesGarbage(t *testing.T) {
	if got := readAgentHookInput(strings.NewReader("")); got.SessionID != "" {
		t.Fatalf("empty input should yield zero struct, got %+v", got)
	}
	if got := readAgentHookInput(strings.NewReader("not json")); got.SessionID != "" {
		t.Fatalf("garbage input should yield zero struct, got %+v", got)
	}
	got := readAgentHookInput(strings.NewReader(`{"session_id":"abc","source":"compact","cwd":"/tmp","stop_hook_active":true,"workspace":{"current_dir":"/w"}}`))
	if got.SessionID != "abc" || got.Source != "compact" || !got.StopHookActive || got.Workspace.CurrentDir != "/w" {
		t.Fatalf("parsed input = %+v", got)
	}
}

func TestEnsureClaudeHookIsIdempotentAndRefreshesPath(t *testing.T) {
	hooks := map[string]any{}
	if !ensureClaudeHook(hooks, "Stop", "", "/old/pane hook stop") {
		t.Fatal("first ensure should add the hook")
	}
	if ensureClaudeHook(hooks, "Stop", "", "/new/pane hook stop") {
		t.Fatal("second ensure should find the existing hook")
	}
	groups := hooks["Stop"].([]any)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	entry := groups[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)
	if entry["command"] != "/new/pane hook stop" {
		t.Fatalf("command not refreshed: %v", entry["command"])
	}
}

func TestInstallClaudeIntegrationPreservesUserSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	existing := `{"model":"opus","statusLine":{"type":"command","command":"my-custom-status"},"permissions":{"allow":["Bash(ls:*)"]}}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := installClaudeIntegration(&out, "/usr/local/bin/pane"); err != nil {
		t.Fatalf("install returned error: %v", err)
	}
	// Twice: must be idempotent.
	if err := installClaudeIntegration(&out, "/usr/local/bin/pane"); err != nil {
		t.Fatalf("second install returned error: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("settings no longer valid JSON: %v", err)
	}
	if settings["model"] != "opus" {
		t.Fatalf("user setting clobbered: model = %v", settings["model"])
	}
	statusLine := settings["statusLine"].(map[string]any)
	if statusLine["command"] != "my-custom-status" {
		t.Fatalf("custom statusline overwritten: %v", statusLine["command"])
	}
	hooks := settings["hooks"].(map[string]any)
	for _, event := range []string{"SessionStart", "Stop", "UserPromptSubmit"} {
		groups, ok := hooks[event].([]any)
		if !ok || len(groups) != 1 {
			t.Fatalf("event %s: expected exactly 1 group, got %v", event, hooks[event])
		}
	}
	sessionStart := hooks["SessionStart"].([]any)[0].(map[string]any)
	if sessionStart["matcher"] != "startup|resume|clear|compact" {
		t.Fatalf("SessionStart matcher = %v", sessionStart["matcher"])
	}
}

func TestInstallClaudeIntegrationInstallsStatuslineWhenAbsent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var out bytes.Buffer
	if err := installClaudeIntegration(&out, "/usr/local/bin/pane"); err != nil {
		t.Fatalf("install returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	statusLine := settings["statusLine"].(map[string]any)
	if statusLine["command"] != "/usr/local/bin/pane statusline" {
		t.Fatalf("statusline command = %v", statusLine["command"])
	}
}

// restoreWorkingDir undoes chdirs done by enterAgentDir during a test, so a
// deleted t.TempDir is never left as the package cwd for later tests.
func restoreWorkingDir(t *testing.T) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})
}

func TestTruncateStatusKeepsShortAndTrimsLong(t *testing.T) {
	if got := truncateStatus("short intent", 40); got != "short intent" {
		t.Fatalf("short intent altered: %q", got)
	}
	long := strings.Repeat("x", 39) + "yz"
	got := truncateStatus(long, 40)
	if got != strings.Repeat("x", 39)+"…" {
		t.Fatalf("long intent = %q", got)
	}
	if len([]rune(got)) != 40 {
		t.Fatalf("truncated length = %d runes", len([]rune(got)))
	}
}

func TestRunStatuslineReportsDaemonOffline(t *testing.T) {
	restoreWorkingDir(t)
	t.Setenv("PANE_SOCKET_PATH", filepath.Join(t.TempDir(), "missing.sock"))
	t.Setenv("PANE_NO_AUTOSTART", "1")
	var out bytes.Buffer
	input := `{"session_id":"abc","cwd":"` + t.TempDir() + `"}`
	if err := runStatusline(nil, strings.NewReader(input), &out); err != nil {
		t.Fatalf("statusline must not error when daemon is down: %v", err)
	}
	if !strings.Contains(out.String(), "daemon offline") {
		t.Fatalf("expected offline notice, got %q", out.String())
	}
}

func TestRunHookStopAllowsStopWhenLoopGuardActive(t *testing.T) {
	t.Setenv("PANE_SOCKET_PATH", filepath.Join(t.TempDir(), "missing.sock"))
	t.Setenv("PANE_NO_AUTOSTART", "1")
	var out bytes.Buffer
	input := agentHookInput{StopHookActive: true}
	if err := runHookStop(input, &out); err != nil {
		t.Fatalf("stop hook errored: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("stop hook must stay silent under loop guard, got %q", out.String())
	}
}

func TestRunHookSessionStartSilentWhenDaemonDown(t *testing.T) {
	restoreWorkingDir(t)
	t.Setenv("PANE_SOCKET_PATH", filepath.Join(t.TempDir(), "missing.sock"))
	t.Setenv("PANE_NO_AUTOSTART", "1")
	var out bytes.Buffer
	input := agentHookInput{SessionID: "abc", CWD: t.TempDir()}
	if err := runHookSessionStart(input, &out); err != nil {
		t.Fatalf("session-start hook must not error when daemon is down: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("session-start hook must stay silent when daemon is down, got %q", out.String())
	}
}
