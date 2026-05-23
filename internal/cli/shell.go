package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/juliancanalez/pane/internal/daemon"
	"github.com/juliancanalez/pane/internal/protocol"
	"github.com/juliancanalez/pane/internal/store"
)

func runShellInit(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: pane shell-init")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	logPath := store.DefaultLogPath(home)
	_, _ = fmt.Fprintf(stdout, shellInitTemplate, strconv.Quote(executable), strconv.Quote(logPath))
	return nil
}

const shellInitTemplate = `# Pane shell integration
export PANE_BIN=%s
export PANE_LOG=${PANE_LOG:-%s}

_pane_start_daemon() {
  "$PANE_BIN" daemon health >/dev/null 2>&1 && return 0
  mkdir -p "$(dirname "$PANE_LOG")"
  "$PANE_BIN" daemon start >>"$PANE_LOG" 2>&1 &
  i=0
  while [ $i -lt 30 ]; do
    "$PANE_BIN" daemon health >/dev/null 2>&1 && return 0
    i=$((i + 1))
    sleep 0.1
  done
  return 1
}

_pane_heartbeat() {
  _pane_start_daemon >/dev/null 2>&1 || return 0
  "$PANE_BIN" heartbeat >/dev/null 2>&1 || return 0
}

_pane_session_start() {
  _pane_heartbeat
  "$PANE_BIN" summary 1>&2 2>/dev/null || true
}

if [ -n "$ZSH_VERSION" ]; then
  autoload -Uz add-zsh-hook 2>/dev/null || true
  if type add-zsh-hook >/dev/null 2>&1; then
    add-zsh-hook precmd _pane_heartbeat
  fi
elif [ -n "$BASH_VERSION" ]; then
  if [ -n "$PROMPT_COMMAND" ]; then
    PROMPT_COMMAND="_pane_heartbeat; $PROMPT_COMMAND"
  else
    PROMPT_COMMAND="_pane_heartbeat"
  fi
fi

_pane_session_start
`

const shellHookMarkerStart = "# >>> pane shell integration >>>"
const shellHookMarkerEnd = "# <<< pane shell integration <<<"

func runSetup(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: pane setup")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	installDir := filepath.Join(home, ".pane", "bin")
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return err
	}
	installedBin := filepath.Join(installDir, "pane")
	if err := copyFile(executable, installedBin, 0o755); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "installed binary: %s\n", installedBin)

	shimPath, err := installGitShim(installedBin, home)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "installed git shim: %s\n", shimPath)

	rcPath, err := defaultShellRC(home)
	if err != nil {
		return err
	}
	if err := installShellHook(rcPath, installedBin); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "installed shell hook: %s\n", rcPath)

	client := daemon.Client{SocketPath: store.DefaultSocketPath(home)}
	if response, err := client.Send(protocol.Request{Type: protocol.RequestDaemonHealth}); err == nil && response.OK {
		_, _ = fmt.Fprintf(stdout, "daemon already running\n")
		return nil
	}
	if err := daemon.StartBackground(store.DefaultSocketPath(home)); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "daemon started\n")
	return nil
}

func runDoctor(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: pane doctor")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	installedBin := filepath.Join(home, ".pane", "bin", "pane")
	checkPath(stdout, "binary", installedBin)
	checkPath(stdout, "database", store.DefaultDBPath(home))
	checkPath(stdout, "socket", store.DefaultSocketPath(home))
	checkPath(stdout, "pid file", store.DefaultPIDPath(home))
	checkPath(stdout, "log", store.DefaultLogPath(home))
	checkPath(stdout, "git shim", filepath.Join(store.DefaultShimDir(home), "git"))
	if rcPath, err := defaultShellRC(home); err == nil {
		checkShellHook(stdout, rcPath)
	}
	client := daemon.Client{SocketPath: store.DefaultSocketPath(home)}
	if response, err := client.Send(protocol.Request{Type: protocol.RequestDaemonHealth}); err == nil && response.OK {
		_, _ = fmt.Fprintf(stdout, "ok daemon running pid=%v uptime_ms=%v\n", response.Payload["pid"], response.Payload["uptime_ms"])
	} else {
		_, _ = fmt.Fprintf(stdout, "warn daemon unreachable: %v\n", err)
	}
	return nil
}

func checkPath(stdout io.Writer, label, path string) {
	if _, err := os.Stat(path); err == nil {
		_, _ = fmt.Fprintf(stdout, "ok %s: %s\n", label, path)
	} else {
		_, _ = fmt.Fprintf(stdout, "warn %s missing: %s\n", label, path)
	}
}

func checkShellHook(stdout io.Writer, rcPath string) {
	content, err := os.ReadFile(rcPath)
	if err == nil && strings.Contains(string(content), shellHookMarkerStart) {
		_, _ = fmt.Fprintf(stdout, "ok shell hook: %s\n", rcPath)
		return
	}
	_, _ = fmt.Fprintf(stdout, "warn shell hook missing: %s\n", rcPath)
}

func defaultShellRC(home string) (string, error) {
	shell := filepath.Base(os.Getenv("SHELL"))
	if shell == "bash" {
		return filepath.Join(home, ".bashrc"), nil
	}
	return filepath.Join(home, ".zshrc"), nil
}

func installShellHook(rcPath, paneBin string) error {
	block := fmt.Sprintf("%s\nexport PATH=%s:\"$PATH\"\neval \"$(%s shell-init)\"\n%s\n", shellHookMarkerStart, strconv.Quote(filepath.Dir(paneBin)), strconv.Quote(paneBin), shellHookMarkerEnd)
	content, err := os.ReadFile(rcPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := string(content)
	if strings.Contains(text, shellHookMarkerStart) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(rcPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(rcPath, []byte(strings.TrimRight(text, "\n")+"\n\n"+block), 0o644)
}

func copyFile(src, dst string, mode os.FileMode) error {
	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, content, mode)
}

func installGitShim(paneBin, home string) (string, error) {
	shimDir := store.DefaultShimDir(home)
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		return "", err
	}
	shimPath := filepath.Join(shimDir, "git")
	content := fmt.Sprintf("#!/bin/sh\nexec %s git \"$@\"\n", strconv.Quote(paneBin))
	if err := os.WriteFile(shimPath, []byte(content), 0o755); err != nil {
		return "", err
	}
	return shimPath, nil
}

func runShims(args []string, stdout io.Writer) error {
	if len(args) != 1 || args[0] != "install" {
		return errors.New("usage: pane shims install")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	shimPath, err := installGitShim(executable, home)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "installed git shim: %s\nadd to PATH: export PATH=\"%s:$PATH\"\n", shimPath, filepath.Dir(shimPath))
	return nil
}
