package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

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
  pane daemon status                Show daemon process, socket, DB, and log paths

  pane init                         Register or resume this terminal pane as a Pane session
  pane heartbeat                    Quietly refresh this pane session's cwd/branch/last-seen state
  pane close                        Close this pane session so it leaves the active board
  pane status                       Show this session's workspace, branch, intent, and state
  pane intent <text>                Record what this session is currently working on
  pane board                        Show the workspace shared awareness board
  pane summary                      Show startup context for this session
  pane continue <session-id>        Link this session to a previous session handoff
  pane history [--since <duration>] Show recent sessions for this workspace
  pane sessions prune              Close stale active/idle sessions in this workspace

  pane shell-init                   Print shell hook for daemon start + session heartbeat
  pane shims install                Install transparent git shim under ~/.pane/shims

  pane daemon health                Check daemon health over the Unix socket
  pane daemon stop                  Ask the daemon to stop cleanly

  pane git <git-args...>            Run git through Pane's shared-state preflight checks

  pane ask <session-id> <message>   Send an async coordination question to another session
  pane inbox                        Show unread coordination messages for this session
  pane reply <message-id> <message> Reply to a coordination thread

  pane state set <key> <json>      Store namespaced JSON state for this workspace
  pane state get <key>             Read workspace state as JSON
  pane state list [prefix]         List workspace state keys and JSON values
  pane state delete <key>          Delete workspace state

Session and board commands require the daemon to be running.

Environment overrides:
  PANE_DB_PATH                      Use a custom SQLite database path
  PANE_SOCKET_PATH                  Use a custom daemon socket path
  PANE_PID_PATH                     Use a custom daemon PID file path
  PANE_LOG_PATH or PANE_LOG         Use a custom daemon log path
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
	case "heartbeat":
		return runHeartbeat(args[1:], stdout)
	case "close":
		return runClose(args[1:], stdout)
	case "sessions":
		return runSessions(args[1:], stdout)
	case "status":
		return runStatus(args[1:], stdout)
	case "board":
		return runBoard(args[1:], stdout)
	case "summary":
		return runSummary(args[1:], stdout)
	case "continue":
		return runContinue(args[1:], stdout)
	case "history":
		return runHistory(args[1:], stdout)
	case "intent":
		return runIntent(args[1:], stdout)
	case "shell-init":
		return runShellInit(args[1:], stdout)
	case "shims":
		return runShims(args[1:], stdout)
	case "git":
		return runGit(args[1:], stdout, stderr)
	case "ask":
		return runAsk(args[1:], stdout)
	case "inbox":
		return runInbox(args[1:], stdout)
	case "reply":
		return runReply(args[1:], stdout)
	case "state":
		return runState(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runDaemon(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: pane daemon start|stop|health|status")
	}
	socket, err := socketPath()
	if err != nil {
		return err
	}
	db, err := databasePath()
	if err != nil {
		return err
	}
	pid, err := pidPath()
	if err != nil {
		return err
	}
	log, err := logPath()
	if err != nil {
		return err
	}
	client := daemon.Client{SocketPath: socket}

	switch args[0] {
	case "start":
		if response, err := client.Send(protocol.Request{Type: protocol.RequestDaemonHealth}); err == nil && response.OK {
			_, _ = fmt.Fprintf(stdout, "daemon already running\npid: %v\nsocket: %s\ndb: %s\nlog: %s\n", response.Payload["pid"], response.Payload["socket_path"], response.Payload["db_path"], response.Payload["log_path"])
			return nil
		}
		_, _ = fmt.Fprintf(stdout, "daemon starting on %s\n", socket)
		return daemon.New(daemon.Config{SocketPath: socket, DBPath: db, PIDPath: pid, LogPath: log}).Run(context.Background())
	case "health":
		response, err := client.Send(protocol.Request{Type: protocol.RequestDaemonHealth})
		if err != nil {
			return err
		}
		if !response.OK {
			return errors.New(response.Error)
		}
		_, _ = fmt.Fprintf(stdout, "daemon: %s\npid: %v\nuptime_ms: %v\nsocket: %s\ndb: %s\npid_file: %s\nlog: %s\n", response.Payload["status"], response.Payload["pid"], response.Payload["uptime_ms"], response.Payload["socket_path"], response.Payload["db_path"], response.Payload["pid_path"], response.Payload["log_path"])
		return nil
	case "status":
		return runDaemonStatus(stdout, client, socket, db, pid, log)
	case "stop":
		response, err := client.Send(protocol.Request{Type: protocol.RequestDaemonStop})
		if err != nil {
			return err
		}
		if !response.OK {
			return errors.New(response.Error)
		}
		_, _ = fmt.Fprintf(stdout, "daemon: %s\n", response.Payload["status"])
		return nil
	default:
		return errors.New("usage: pane daemon start|stop|health|status")
	}
}

func runDaemonStatus(stdout io.Writer, client daemon.Client, socket, db, pid, log string) error {
	response, err := client.Send(protocol.Request{Type: protocol.RequestDaemonHealth})
	if err == nil && response.OK {
		_, _ = fmt.Fprintf(stdout, "daemon: running\npid: %v\nuptime_ms: %v\nsocket: %s\ndb: %s\npid_file: %s\nlog: %s\n", response.Payload["pid"], response.Payload["uptime_ms"], response.Payload["socket_path"], response.Payload["db_path"], response.Payload["pid_path"], response.Payload["log_path"])
		return nil
	}
	_, _ = fmt.Fprintf(stdout, "daemon: stopped\nsocket: %s\ndb: %s\npid_file: %s\nlog: %s\n", socket, db, pid, log)
	if contents, readErr := os.ReadFile(pid); readErr == nil {
		_, _ = fmt.Fprintf(stdout, "stale_pid_file: %s", string(contents))
	}
	return nil
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

func runHeartbeat(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: pane heartbeat")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionHeartbeat, Payload: protocol.EnvironmentPayload(env)})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprintf(stdout, "heartbeat: %s\n", payloadString(response, "session_id"))
	return nil
}

func runClose(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: pane close")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionClose, Payload: protocol.EnvironmentPayload(env)})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprintf(stdout, "session closed: %s\n", payloadString(response, "session_id"))
	return nil
}

func runSessions(args []string, stdout io.Writer) error {
	if len(args) != 1 || args[0] != "prune" {
		return errors.New("usage: pane sessions prune")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionPrune, Payload: protocol.BoardRequestPayload(env)})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprintf(stdout, "stale sessions closed: %v\n", response.Payload["closed"])
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

func runSummary(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: pane summary")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestGetSummary, Payload: protocol.EnvironmentPayload(env)})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprint(stdout, payloadString(response, "text"))
	return nil
}

func runContinue(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: pane continue <session-id>")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionContinue, Payload: protocol.ContinuePayload(env, args[0])})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	state := "created"
	if payloadBool(response, "resumed") {
		state = "linked"
	}
	_, _ = fmt.Fprintf(stdout, "session %s: %s\ncontinued from: %s\nbranch: %s\nworkspace: %s\n", state, payloadString(response, "session_id"), payloadString(response, "parent_session_id"), payloadString(response, "branch"), payloadString(response, "workspace_root"))
	return nil
}

func runHistory(args []string, stdout io.Writer) error {
	since, err := parseHistorySince(args)
	if err != nil {
		return err
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionHistory, Payload: protocol.HistoryRequestPayload(env, since)})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprint(stdout, payloadString(response, "text"))
	return nil
}

func parseHistorySince(args []string) (int64, error) {
	if len(args) == 0 {
		return 0, nil
	}
	if len(args) != 2 || args[0] != "--since" {
		return 0, errors.New("usage: pane history [--since <duration>]")
	}
	duration, err := time.ParseDuration(args[1])
	if err != nil {
		return 0, fmt.Errorf("invalid --since duration: %w", err)
	}
	return time.Now().Add(-duration).Unix(), nil
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
	env, envErr := session.DetectEnvironment()
	if envErr == nil && gitguard.Parse(args).Watched {
		response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestGitPreflight, Payload: protocol.GitRequestPayload(env, args)})
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "[Pane] daemon unavailable, running git without coordination: %v\n", err)
		} else if !response.OK {
			_, _ = fmt.Fprintf(stderr, "[Pane] git preflight failed, running git without coordination: %s\n", response.Error)
		} else {
			for _, warning := range response.Warnings {
				_, _ = fmt.Fprintf(stderr, "[Pane] Warning: %s\n", warning)
			}
			if response.Block {
				return errors.New("git command blocked by Pane preflight")
			}
		}
	}

	code, err := runRealGit(args, stdout, stderr)
	result := "ok"
	if err != nil {
		result = fmt.Sprintf("exit:%d", code)
	}
	if envErr == nil {
		_, _ = sendDaemonRequest(protocol.Request{Type: protocol.RequestGitRecord, Payload: protocol.GitRecordPayload(env, args, result)})
	}
	if err != nil {
		return commandExitError{code: code}
	}
	return nil
}

func runState(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: pane state set|get|list|delete ...")
	}
	switch args[0] {
	case "set":
		return runStateSet(args[1:], stdout)
	case "get":
		return runStateGet(args[1:], stdout)
	case "list":
		return runStateList(args[1:], stdout)
	case "delete":
		return runStateDelete(args[1:], stdout)
	default:
		return errors.New("usage: pane state set|get|list|delete ...")
	}
}

func runStateSet(args []string, stdout io.Writer) error {
	if len(args) != 2 {
		return errors.New("usage: pane state set <key> <json>")
	}
	if !json.Valid([]byte(args[1])) {
		return errors.New("state value must be valid JSON")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestStateSet, Payload: protocol.StateRequestPayload(env, args[0], args[1], "")})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprintf(stdout, "state set: %s\n", payloadString(response, "key"))
	return nil
}

func runStateGet(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: pane state get <key>")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestStateGet, Payload: protocol.StateRequestPayload(env, args[0], "", "")})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprintf(stdout, "%s\n", payloadString(response, "value_json"))
	return nil
}

func runStateList(args []string, stdout io.Writer) error {
	if len(args) > 1 {
		return errors.New("usage: pane state list [prefix]")
	}
	prefix := ""
	if len(args) == 1 {
		prefix = args[0]
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestStateList, Payload: protocol.StateRequestPayload(env, "", "", prefix)})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	for _, item := range payloadMaps(response, "items") {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\n", mapString(item, "key"), mapString(item, "value_json"))
	}
	return nil
}

func runStateDelete(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: pane state delete <key>")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestStateDelete, Payload: protocol.StateRequestPayload(env, args[0], "", "")})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprintf(stdout, "state deleted: %s\n", payloadString(response, "key"))
	return nil
}

func runAsk(args []string, stdout io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: pane ask <session-id> <message>")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestMessageSend, Payload: protocol.MessageSendRequestPayload(env, args[0], strings.Join(args[1:], " "))})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprintf(stdout, "message sent: %s\nthread: %s\nto: %s\n", payloadString(response, "message_id"), payloadString(response, "thread_id"), payloadString(response, "to_session"))
	return nil
}

func runInbox(args []string, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: pane inbox")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestMessageList, Payload: protocol.EnvironmentPayload(env)})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprint(stdout, payloadString(response, "text"))
	return nil
}

func runReply(args []string, stdout io.Writer) error {
	if len(args) < 2 {
		return errors.New("usage: pane reply <message-id> <message>")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestMessageReply, Payload: protocol.MessageReplyRequestPayload(env, args[0], strings.Join(args[1:], " "))})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprintf(stdout, "reply sent: %s\nthread: %s\nto: %s\n", payloadString(response, "message_id"), payloadString(response, "thread_id"), payloadString(response, "to_session"))
	return nil
}
