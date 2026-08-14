package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/juliancanalez/pane/internal/daemon"
	"github.com/juliancanalez/pane/internal/protocol"
	"github.com/juliancanalez/pane/internal/store"
	"github.com/juliancanalez/pane/internal/version"
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
if [ -z "$PANE_TTY" ]; then
  PANE_TTY="$(tty 2>/dev/null)" && export PANE_TTY || unset PANE_TTY
fi

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

type setupOptions struct {
	InstallShell  bool
	InstallShim   bool
	StartDaemon   bool
	PrintShell    bool
	InstallClaude bool
}

func parseSetupOptions(args []string) (setupOptions, error) {
	options := setupOptions{InstallShell: true, InstallShim: true, StartDaemon: true}
	for _, arg := range args {
		switch arg {
		case "--no-shell":
			options.InstallShell = false
		case "--no-shim":
			options.InstallShim = false
		case "--no-daemon":
			options.StartDaemon = false
		case "--print-shell":
			options.PrintShell = true
			options.InstallShell = false
		case "--claude":
			options.InstallClaude = true
		default:
			return setupOptions{}, errors.New("usage: pane setup [--no-shell] [--no-shim] [--no-daemon] [--print-shell] [--claude]")
		}
	}
	return options, nil
}

func runSetup(args []string, stdout io.Writer) error {
	options, err := parseSetupOptions(args)
	if err != nil {
		return err
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
	if err := installExecutable(executable, installedBin, 0o755); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "installed binary: %s\n", installedBin)
	if analyzerSource, err := findAnalyzerSource(executable); err == nil {
		installedAnalyzer := filepath.Join(installDir, "pane-analyze")
		if err := installExecutable(analyzerSource, installedAnalyzer, 0o755); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "installed analyzer: %s\n", installedAnalyzer)
	} else {
		_, _ = fmt.Fprintf(stdout, "warn analyzer not installed: %v\n", err)
	}

	if options.InstallShim {
		shimPath, err := installGitShim(installedBin, home)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "installed git shim: %s\n", shimPath)
	} else {
		_, _ = fmt.Fprintf(stdout, "skipped git shim\n")
	}

	if options.PrintShell {
		rcPath, err := defaultShellRC(home)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "shell hook for %s:\n%s", rcPath, shellHookBlock(installedBin))
	} else if options.InstallShell {
		rcPath, err := defaultShellRC(home)
		if err != nil {
			return err
		}
		if err := installShellHook(rcPath, installedBin); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "installed shell hook: %s\n", rcPath)
	} else {
		_, _ = fmt.Fprintf(stdout, "skipped shell hook\n")
	}

	if options.InstallClaude {
		if err := installClaudeIntegration(stdout, installedBin); err != nil {
			return err
		}
	}

	if !options.StartDaemon {
		_, _ = fmt.Fprintf(stdout, "skipped daemon start\n")
		return nil
	}
	socket := store.DefaultSocketPath(home)
	client := daemon.Client{SocketPath: socket}
	if response, err := client.Send(protocol.Request{Type: protocol.RequestDaemonHealth}); err == nil && response.OK {
		if version.IsOlder(response.DaemonVersion, version.Version) {
			if err := daemon.Restart(socket); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(stdout, "daemon restarted with %s\n", version.Version)
			return nil
		}
		_, _ = fmt.Fprintf(stdout, "daemon already running\n")
		return nil
	}
	if err := daemon.StartBackground(socket); err != nil {
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
	_, _ = fmt.Fprintf(stdout, "cli version: %s\n", version.Version)
	installedBin := filepath.Join(home, ".pane", "bin", "pane")
	checkPath(stdout, "binary", installedBin)
	checkPath(stdout, "analyzer", filepath.Join(home, ".pane", "bin", "pane-analyze"))
	checkRuntimeAnalyzer(stdout)
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
		_, _ = fmt.Fprintf(stdout, "ok daemon running version=%s pid=%v uptime_ms=%v\n", displayDaemonVersion(response), response.Payload["pid"], response.Payload["uptime_ms"])
		if version.IsOlder(response.DaemonVersion, version.Version) {
			_, _ = fmt.Fprintf(stdout, "warn daemon older than CLI; it restarts automatically on the next pane command\n")
		}
	} else {
		_, _ = fmt.Fprintf(stdout, "warn daemon unreachable: %v\n", err)
	}
	return nil
}

func checkRuntimeAnalyzer(stdout io.Writer) {
	executable, _ := os.Executable()
	path, err := findAnalyzerSource(executable)
	if err != nil {
		_, _ = fmt.Fprintf(stdout, "warn analyzer runtime unavailable: %v\n", err)
		return
	}
	_, _ = fmt.Fprintf(stdout, "ok analyzer runtime: %s\n", path)
}

func findAnalyzerSource(executable string) (string, error) {
	candidates := []string{}
	if value := os.Getenv("PANE_ANALYZER_PATH"); value != "" {
		candidates = append(candidates, value)
	}
	if executable != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "pane-analyze"))
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "bin", "pane-analyze"))
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", errors.New("pane-analyze not found; run `make build` or set PANE_ANALYZER_PATH")
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

func shellHookBlock(paneBin string) string {
	return fmt.Sprintf("%s\nexport PATH=%s:\"$PATH\"\neval \"$(%s shell-init)\"\n%s\n", shellHookMarkerStart, strconv.Quote(filepath.Dir(paneBin)), strconv.Quote(paneBin), shellHookMarkerEnd)
}

func installShellHook(rcPath, paneBin string) error {
	block := shellHookBlock(paneBin)
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

// installExecutable copies a binary and, on macOS, re-signs it ad-hoc.
// Rewriting the bytes of a binary that carries Go's linker ad-hoc signature
// invalidates that signature on Apple Silicon, and the kernel then SIGKILLs the
// copy. Re-signing makes a source/HEAD install runnable. Best effort: if
// codesign is unavailable the copy still happens (it may already be valid, e.g.
// for a brew-bottled binary).
func installExecutable(src, dst string, mode os.FileMode) error {
	if err := copyFile(src, dst, mode); err != nil {
		return err
	}
	if runtime.GOOS == "darwin" {
		if path, err := exec.LookPath("codesign"); err == nil {
			_ = exec.Command(path, "--force", "--sign", "-", dst).Run()
		}
	}
	return nil
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
