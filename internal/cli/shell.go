package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

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
	shimDir := store.DefaultShimDir(home)
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		return err
	}
	shimPath := filepath.Join(shimDir, "git")
	content := fmt.Sprintf("#!/bin/sh\nexec %s git \"$@\"\n", strconv.Quote(executable))
	if err := os.WriteFile(shimPath, []byte(content), 0o755); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "installed git shim: %s\nadd to PATH: export PATH=\"%s:$PATH\"\n", shimPath, shimDir)
	return nil
}
