package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/juliancanalez/pane/internal/daemon"
	"github.com/juliancanalez/pane/internal/gitguard"
	"github.com/juliancanalez/pane/internal/protocol"
	"github.com/juliancanalez/pane/internal/session"
)

const usage = `Pane gives concurrent coding agents shared local memory over a workspace.
It tracks pane-based sessions, remembers recent activity, routes context between
sessions, and adds guardrails at high-risk moments.

Usage:
  pane daemon start                 Start the local Pane shared-memory daemon

  pane init                         Register or resume this terminal pane as a Pane session
  pane status                       Show this session's workspace, branch, intent, and state
  pane intent <text>                Record what this session is currently working on
  pane board                        Show the workspace shared awareness board
  pane summary                      Show startup context for this session (not implemented yet)

  pane daemon health                Check daemon health over the Unix socket
  pane daemon stop                  Ask the daemon to stop cleanly

  pane git <git-args...>            Run git through Pane's shared-state preflight checks

  pane ask <session-id> <message>   Send an async coordination question to another session
  pane inbox                        Show unread coordination messages for this session
  pane reply <message-id> <message> Reply to a coordination thread

Session and board commands require the daemon to be running.

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
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionInit, Payload: protocol.EnvironmentPayload(env)})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	state := "created"
	if payloadBool(response, "resumed") {
		state = "resumed"
	}
	_, _ = fmt.Fprintf(stdout, "session %s: %s\nbranch: %s\nworkspace: %s\n", state, payloadString(response, "session_id"), payloadString(response, "branch"), payloadString(response, "workspace_root"))
	return nil
}

func runBoard(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: pane board")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestGetBoard, Payload: protocol.BoardRequestPayload(env)})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprint(stdout, payloadString(response, "text"))
	return nil
}

func runStatus(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: pane status")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionStatus, Payload: protocol.EnvironmentPayload(env)})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprintf(stdout, "session: %s\nstatus: %s\nbranch: %s\nintent: %s\nworkspace: %s\n", payloadString(response, "session_id"), payloadString(response, "status"), payloadString(response, "branch"), payloadString(response, "intent"), payloadString(response, "workspace_root"))
	return nil
}

func runIntent(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: pane intent <text>")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	intent := strings.Join(args, " ")
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionIntent, Payload: protocol.IntentPayload(env, intent)})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprintf(stdout, "intent updated: %s\n", payloadString(response, "intent"))
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
