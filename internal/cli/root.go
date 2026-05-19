package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/juliancanalez/pane/internal/board"
	"github.com/juliancanalez/pane/internal/daemon"
	"github.com/juliancanalez/pane/internal/gitguard"
	"github.com/juliancanalez/pane/internal/protocol"
	"github.com/juliancanalez/pane/internal/session"
)

const usage = `Pane gives concurrent coding agents shared local memory over a workspace.
It tracks pane-based sessions, remembers recent activity, routes context between
sessions, and adds guardrails at high-risk moments.

Usage:
  pane init                         Register or resume this terminal pane as a Pane session
  pane status                       Show this session's workspace, branch, intent, and state
  pane intent <text>                Record what this session is currently working on
  pane board                        Show the workspace shared awareness board
  pane summary                      Show startup context for this session (not implemented yet)

  pane daemon start                 Start the local Pane shared-memory daemon
  pane daemon health                Check daemon health over the Unix socket
  pane daemon stop                  Ask the daemon to stop cleanly

  pane git <git-args...>            Run git through Pane's shared-state preflight checks

  pane ask <session-id> <message>   Send an async coordination question to another session
  pane inbox                        Show unread coordination messages for this session
  pane reply <message-id> <message> Reply to a coordination thread

Environment overrides:
  PANE_DB_PATH                      Use a custom SQLite database path
  PANE_SOCKET_PATH                  Use a custom daemon socket path
`

func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		_, _ = fmt.Fprint(stdout, usage)
		return nil
	}

	switch args[0] {
	case "daemon":
		return runDaemon(args[1:], stdout)
	case "init":
		return runInit(args[1:], stdout)
	case "status":
		return runStatus(args[1:], stdout)
	case "board":
		return runBoard(args[1:], stdout)
	case "summary":
		return runSessionCommand("summary", args[1:], stdout)
	case "intent":
		return runIntent(args[1:], stdout)
	case "git":
		return runGit(args[1:], stdout, stderr)
	case "ask":
		return runAsk(args[1:], stdout)
	case "inbox":
		return runInbox(args[1:], stdout)
	case "reply":
		return runReply(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runDaemon(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: pane daemon start|stop|health")
	}
	socket, err := socketPath()
	if err != nil {
		return err
	}
	db, err := databasePath()
	if err != nil {
		return err
	}

	switch args[0] {
	case "start":
		_, _ = fmt.Fprintf(stdout, "daemon starting on %s\n", socket)
		return daemon.New(daemon.Config{SocketPath: socket, DBPath: db}).Run(context.Background())
	case "health":
		response, err := daemon.Client{SocketPath: socket}.Send(protocol.Request{Type: protocol.RequestDaemonHealth})
		if err != nil {
			return err
		}
		if !response.OK {
			return errors.New(response.Error)
		}
		_, _ = fmt.Fprintf(stdout, "daemon: %s\nuptime_ms: %.0f\nsocket: %s\n", response.Payload["status"], response.Payload["uptime_ms"], response.Payload["socket_path"])
		return nil
	case "stop":
		response, err := daemon.Client{SocketPath: socket}.Send(protocol.Request{Type: protocol.RequestDaemonStop})
		if err != nil {
			return err
		}
		if !response.OK {
			return errors.New(response.Error)
		}
		_, _ = fmt.Fprintf(stdout, "daemon: %s\n", response.Payload["status"])
		return nil
	default:
		return errors.New("usage: pane daemon start|stop|health")
	}
}

func runSessionCommand(name string, args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: pane %s", name)
	}
	_, _ = fmt.Fprintf(stdout, "%s: not implemented yet\n", name)
	return nil
}

func runInit(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: pane init")
	}
	env, manager, cleanup, err := sessionRuntime()
	if err != nil {
		return err
	}
	defer cleanup()

	result, err := manager.Init(context.Background(), session.InitInput{
		PaneID:        env.PaneID,
		TTY:           env.TTY,
		WorkspaceRoot: env.WorkspaceRoot,
		CWD:           env.CWD,
		Branch:        env.Branch,
	})
	if err != nil {
		return err
	}
	state := "created"
	if result.Resumed {
		state = "resumed"
	}
	_, _ = fmt.Fprintf(stdout, "session %s: %s\nbranch: %s\nworkspace: %s\n", state, result.Session.ID, result.Session.Branch, result.Session.WorkspaceRoot)
	return nil
}

func runBoard(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: pane board")
	}
	env, manager, cleanup, err := sessionRuntime()
	if err != nil {
		return err
	}
	defer cleanup()

	sessions, err := manager.ListActive(context.Background(), env.WorkspaceRoot)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprint(stdout, board.Render(board.FromSessions(env.WorkspaceRoot, sessions), now()))
	return nil
}

func runStatus(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: pane status")
	}
	env, manager, cleanup, err := sessionRuntime()
	if err != nil {
		return err
	}
	defer cleanup()

	current, err := manager.Status(context.Background(), env.PaneID, env.WorkspaceRoot)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return errors.New("no Pane session found for this pane/workspace; run `pane init` first")
		}
		return err
	}
	_, _ = fmt.Fprintf(stdout, "session: %s\nstatus: %s\nbranch: %s\nintent: %s\nworkspace: %s\n", current.ID, current.Status, current.Branch, current.LastIntent, current.WorkspaceRoot)
	return nil
}

func runIntent(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: pane intent <text>")
	}
	env, manager, cleanup, err := sessionRuntime()
	if err != nil {
		return err
	}
	defer cleanup()

	current, err := manager.Status(context.Background(), env.PaneID, env.WorkspaceRoot)
	if err != nil {
		if errors.Is(err, session.ErrNotFound) {
			return errors.New("no Pane session found for this pane/workspace; run `pane init` first")
		}
		return err
	}
	intent := strings.Join(args, " ")
	if err := manager.SetIntent(context.Background(), current.ID, intent); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "intent updated: %s\n", intent)
	return nil
}

func runGit(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: pane git <git-args...>")
	}
	intent := gitguard.Parse(args)
	if intent.Watched {
		_, _ = fmt.Fprintf(stderr, "[Pane] git preflight for %s: not implemented yet\n", intent.Subcommand)
	}
	_, _ = fmt.Fprintf(stdout, "git passthrough: not implemented yet (%s)\n", strings.Join(args, " "))
	return nil
}

func runAsk(args []string, stdout io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: pane ask <session-id> <message>")
	}
	_, _ = fmt.Fprintf(stdout, "ask: not implemented yet (to %s)\n", args[0])
	return nil
}

func runInbox(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: pane inbox")
	}
	_, _ = fmt.Fprintln(stdout, "inbox: not implemented yet")
	return nil
}

func runReply(args []string, stdout io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: pane reply <message-id> <message>")
	}
	_, _ = fmt.Fprintf(stdout, "reply: not implemented yet (to %s)\n", args[0])
	return nil
}
