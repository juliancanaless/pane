package daemon

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// startTestDaemon runs a daemon on socketPath until the returned stop func is
// called.
func startTestDaemon(t *testing.T, socketPath string) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- New(Config{SocketPath: socketPath}).Run(ctx) }()
	waitHealthy(t, socketPath)
	return func() {
		cancel()
		<-done
	}
}

func TestLaunchAgentPlist(t *testing.T) {
	plist := LaunchAgentPlist("/Users/x/.pane/bin/pane", "/Users/x/.pane/logs/pane.log")
	for _, want := range []string{
		"<string>com.pane.daemon</string>",
		"<string>/Users/x/.pane/bin/pane</string>",
		"<string>daemon</string>",
		"<string>--foreground</string>",
		"<key>SuccessfulExit</key>\n\t\t<false/>",
		"<string>/Users/x/.pane/logs/pane.log</string>",
	} {
		if !strings.Contains(plist, want) {
			t.Errorf("plist missing %q:\n%s", want, plist)
		}
	}
}

func TestLaunchAgentPlistPath(t *testing.T) {
	got := LaunchAgentPlistPath("/Users/x")
	want := filepath.Join("/Users/x", "Library", "LaunchAgents", "com.pane.daemon.plist")
	if got != want {
		t.Fatalf("plist path = %q, want %q", got, want)
	}
}

func TestStartBackgroundDefersToLaunchd(t *testing.T) {
	// A daemon already listening plays the part of launchd having started its
	// job; StartBackground must kickstart instead of spawning a competitor.
	socketPath := shortSocketPath(t)
	stopDaemon := startTestDaemon(t, socketPath)
	defer stopDaemon()

	origLoaded, origKickstart := launchAgentLoaded, kickstartLaunchAgent
	t.Cleanup(func() { launchAgentLoaded, kickstartLaunchAgent = origLoaded, origKickstart })
	launchAgentLoaded = func() bool { return true }
	kickstarted := false
	kickstartLaunchAgent = func() error { kickstarted = true; return nil }

	if err := StartBackground(socketPath, ""); err != nil {
		t.Fatalf("StartBackground via launchd: %v", err)
	}
	if !kickstarted {
		t.Fatal("expected launchd kickstart, not a direct spawn")
	}
}

func TestInstallLaunchAgentIsIdempotent(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd is macOS-only")
	}
	home := t.TempDir()
	origLoaded, origRun := launchAgentLoaded, runLaunchctl
	t.Cleanup(func() { launchAgentLoaded, runLaunchctl = origLoaded, origRun })
	launchAgentLoaded = func() bool { return true }
	var calls []string
	runLaunchctl = func(args ...string) error {
		calls = append(calls, args[0])
		return nil
	}

	// Existing correct plist + loaded agent: nothing to do, no daemon bounce.
	plistPath := LaunchAgentPlistPath(home)
	if err := os.MkdirAll(filepath.Dir(plistPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(plistPath, []byte(LaunchAgentPlist("/bin/pane", "/log")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InstallLaunchAgent("/bin/pane", filepath.Join(home, "none.sock"), "/log", home); err != nil {
		t.Fatalf("idempotent install: %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("expected no launchctl calls for an unchanged agent, got %v", calls)
	}

	// A changed plist re-registers: bootout then bootstrap.
	if err := InstallLaunchAgent("/bin/pane2", filepath.Join(home, "none.sock"), "/log", home); err != nil {
		t.Fatalf("reinstall: %v", err)
	}
	if len(calls) != 2 || calls[0] != "bootout" || calls[1] != "bootstrap" {
		t.Fatalf("expected bootout then bootstrap, got %v", calls)
	}
	updated, err := os.ReadFile(plistPath)
	if err != nil || !strings.Contains(string(updated), "/bin/pane2") {
		t.Fatalf("plist not rewritten: %v\n%s", err, updated)
	}
}

// shortSocketPath returns a socket path under /tmp short enough for the
// ~104-byte sun_path limit on macOS; t.TempDir can exceed it.
func shortSocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "pane")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return filepath.Join(dir, "d.sock")
}
