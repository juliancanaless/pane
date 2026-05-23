package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/juliancanalez/pane/internal/analysis"
	"github.com/juliancanalez/pane/internal/daemon"
	"github.com/juliancanalez/pane/internal/gitguard"
	"github.com/juliancanalez/pane/internal/protocol"
	"github.com/juliancanalez/pane/internal/session"
	"github.com/juliancanalez/pane/internal/store"
)

const usage = `Pane gives concurrent coding agents shared local memory over a workspace.
It tracks pane-based sessions, remembers recent activity, routes context between
sessions, and adds guardrails at high-risk moments.

Usage:
  pane daemon start [--background]   Start the local Pane shared-memory daemon
  pane daemon status                Show daemon process, socket, DB, and log paths

  pane init                         Register or resume this terminal pane as a Pane session
  pane heartbeat                    Quietly refresh this pane session's cwd/branch/last-seen state
  pane close                        Close this pane session so it leaves the active board
  pane status                       Show this session's workspace, branch, intent, and state
  pane intent <text>                Record what this session is currently working on
  pane name <name>                  Give this session a human-friendly name for targeting
  pane board [--all] [--repo]       Show the workspace or same-repository awareness board
  pane summary                      Show startup context for this session
  pane continue <session-id>        Link this session to a previous session handoff
  pane spawn <command> [args...]    Run a command as a child Pane session
  pane history [--since <duration>] [--repo] [--lineage] [--format work-log]
                                    Show recent sessions for this workspace or repository
  pane sessions prune              Close stale active/idle sessions in this workspace

  pane shell-init                   Print shell hook for daemon start + session heartbeat
  pane shims install                Install transparent git shim under ~/.pane/shims

  pane daemon health                Check daemon health over the Unix socket
  pane daemon stop                  Ask the daemon to stop cleanly
  pane setup [--no-shell] [--no-shim] [--no-daemon] [--print-shell]
                                    Install local binary, integrations, and start daemon
  pane doctor                       Diagnose local Pane installation and daemon health

  pane git <git-args...>            Run git through Pane's shared-state preflight checks

  pane ask <target> <message>        Send an async coordination question to another session
                                     Target can be a session name, short ID, or full ID
  pane inbox                        Show unread coordination messages for this session
  pane reply <message-id> <message> Reply to a coordination thread

  pane state set [--global] <key> <json>
                                    Store namespaced JSON state
  pane state get [--global] <key>  Read state as JSON
  pane state list [--global] [prefix]
                                    List state keys, owners, and JSON values
  pane state namespaces [--global] Show state namespaces, key counts, and owners
  pane state delete [--global] <key>
                                    Delete state

  pane analyze symbols <file>      Parse a source file and print a JSON symbol table
  pane analyze deps <file>         Parse imports/use/require edges for a source file
  pane analyze index <path...>     Persist symbols and dependency edges in SQLite
  pane analyze dependents <target> Show files with dependency edges to a target/module/symbol

Session and board commands require the daemon to be running.

Environment overrides:
  PANE_DB_PATH                      Use a custom SQLite database path
  PANE_SOCKET_PATH                  Use a custom daemon socket path
  PANE_PID_PATH                     Use a custom daemon PID file path
  PANE_LOG_PATH or PANE_LOG         Use a custom daemon log path
  PANE_PANE_ID                      Override detected terminal-pane identity
  PANE_PARENT_SESSION_ID            Link newly initialized session to a parent
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
	case "spawn":
		return runSpawn(args[1:], stdout, stderr)
	case "history":
		return runHistory(args[1:], stdout)
	case "intent":
		return runIntent(args[1:], stdout)
	case "name":
		return runName(args[1:], stdout)
	case "shell-init":
		return runShellInit(args[1:], stdout)
	case "shims":
		return runShims(args[1:], stdout)
	case "setup":
		return runSetup(args[1:], stdout)
	case "doctor":
		return runDoctor(args[1:], stdout)
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
	case "analyze":
		return runAnalyze(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runDaemon(args []string, stdout io.Writer) error {
	if len(args) < 1 {
		return errors.New("usage: pane daemon start [--background]|stop|health|status")
	}
	subcmd := args[0]
	background := false
	for _, arg := range args[1:] {
		if arg == "--background" || arg == "-b" {
			background = true
		} else {
			return fmt.Errorf("unknown flag: %s", arg)
		}
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

	switch subcmd {
	case "start":
		if response, err := client.Send(protocol.Request{Type: protocol.RequestDaemonHealth}); err == nil && response.OK {
			_, _ = fmt.Fprintf(stdout, "daemon already running\npid: %v\nsocket: %s\ndb: %s\nlog: %s\n", response.Payload["pid"], response.Payload["socket_path"], response.Payload["db_path"], response.Payload["log_path"])
			return nil
		}
		if background {
			_, _ = fmt.Fprintf(stdout, "starting daemon in background...\n")
			if err := daemon.StartBackground(socket); err != nil {
				return err
			}
			_, _ = fmt.Fprintf(stdout, "daemon started\nsocket: %s\ndb: %s\nlog: %s\n", socket, db, log)
			return nil
		}
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
		return errors.New("usage: pane daemon start [--background]|stop|health|status")
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
	showAll := false
	repoScope := false
	for _, arg := range args {
		switch arg {
		case "--all", "-a":
			showAll = true
		case "--repo":
			repoScope = true
		default:
			return errors.New("usage: pane board [--all] [--repo]")
		}
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	reqType := protocol.RequestGetBoard
	payload := protocol.BoardRequestPayload(env)
	if showAll {
		payload["show_all"] = true
	}
	if repoScope {
		payload["scope"] = "repo"
	}
	response, err := sendDaemonRequest(protocol.Request{Type: reqType, Payload: payload})
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

func runSpawn(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: pane spawn <command> [args...]")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	current, err := currentOrInitializedSession(env)
	if err != nil {
		return err
	}

	childEnv := env
	childEnv.PaneID = "spawn:" + randomToken(12)
	childEnv.ParentSessionID = payloadString(current, "session_id")
	childResponse, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionInit, Payload: protocol.EnvironmentPayload(childEnv)})
	if err != nil {
		return err
	}
	if !childResponse.OK {
		return errors.New(childResponse.Error)
	}
	childSessionID := payloadString(childResponse, "session_id")
	_, _ = fmt.Fprintf(stdout, "child session: %s\nparent session: %s\n", childSessionID, childEnv.ParentSessionID)

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Env = withEnv(os.Environ(), map[string]string{
		"PANE_PANE_ID":           childEnv.PaneID,
		"PANE_PARENT_SESSION_ID": childEnv.ParentSessionID,
		"PANE_SESSION_ID":        childSessionID,
	})
	runErr := cmd.Run()

	closeResponse, closeErr := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionClose, Payload: protocol.EnvironmentPayload(childEnv)})
	if closeErr != nil {
		_, _ = fmt.Fprintf(stderr, "[Pane] failed to close child session: %v\n", closeErr)
	} else if !closeResponse.OK {
		_, _ = fmt.Fprintf(stderr, "[Pane] failed to close child session: %s\n", closeResponse.Error)
	}
	return runErr
}

func currentOrInitializedSession(env session.Environment) (protocol.Response, error) {
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionStatus, Payload: protocol.EnvironmentPayload(env)})
	if err == nil && response.OK {
		return response, nil
	}
	response, err = sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionInit, Payload: protocol.EnvironmentPayload(env)})
	if err != nil {
		return protocol.Response{}, err
	}
	if !response.OK {
		return protocol.Response{}, errors.New(response.Error)
	}
	return response, nil
}

func randomToken(bytesLen int) string {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func withEnv(base []string, overrides map[string]string) []string {
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	filtered := make([]string, 0, len(base)+len(overrides))
	for _, item := range base {
		keep := true
		for _, key := range keys {
			if strings.HasPrefix(item, key+"=") {
				keep = false
				break
			}
		}
		if keep {
			filtered = append(filtered, item)
		}
	}
	for key, value := range overrides {
		filtered = append(filtered, key+"="+value)
	}
	return filtered
}

func runHistory(args []string, stdout io.Writer) error {
	since, repoScope, lineage, format, err := parseHistoryArgs(args)
	if err != nil {
		return err
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	payload := protocol.HistoryRequestPayload(env, since)
	if repoScope {
		payload["scope"] = "repo"
	}
	if lineage {
		payload["lineage"] = true
	}
	if format != "" {
		payload["format"] = format
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionHistory, Payload: payload})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprint(stdout, payloadString(response, "text"))
	return nil
}

func parseHistoryArgs(args []string) (int64, bool, bool, string, error) {
	var since int64
	repoScope := false
	lineage := false
	format := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--repo":
			repoScope = true
		case "--lineage":
			lineage = true
		case "--format":
			if i+1 >= len(args) {
				return 0, false, false, "", errors.New("usage: pane history [--since <duration>] [--repo] [--lineage] [--format work-log]")
			}
			format = args[i+1]
			if format != "work-log" {
				return 0, false, false, "", errors.New("unsupported history format: " + format)
			}
			i++
		case "--since":
			if i+1 >= len(args) {
				return 0, false, false, "", errors.New("usage: pane history [--since <duration>] [--repo] [--lineage] [--format work-log]")
			}
			duration, err := time.ParseDuration(args[i+1])
			if err != nil {
				return 0, false, false, "", fmt.Errorf("invalid --since duration: %w", err)
			}
			since = time.Now().Add(-duration).Unix()
			i++
		default:
			return 0, false, false, "", errors.New("usage: pane history [--since <duration>] [--repo] [--lineage] [--format work-log]")
		}
	}
	return since, repoScope, lineage, format, nil
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
	_, _ = fmt.Fprintf(stdout, "session: %s\nstatus: %s\nbranch: %s\nintent: %s\nworkspace: %s\nrepo: %s\n", payloadString(response, "session_id"), payloadString(response, "status"), payloadString(response, "branch"), payloadString(response, "intent"), payloadString(response, "workspace_root"), payloadString(response, "repo_id"))
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

func runName(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: pane name <name>")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	name := strings.Join(args, " ")
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionName, Payload: protocol.NamePayload(env, name)})
	if err != nil {
		return err
	}
	if !response.OK {
		return errors.New(response.Error)
	}
	_, _ = fmt.Fprintf(stdout, "name set: %s\n", payloadString(response, "name"))
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
				_, _ = fmt.Fprintf(stderr, "[Pane] This operation is risky given current shared state.\n")
				if !confirmProceed(stderr) {
					return errors.New("git command cancelled by user")
				}
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

func runAnalyze(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: pane analyze symbols|deps|index|dependents ...")
	}
	switch args[0] {
	case "symbols":
		if len(args) != 2 {
			return errors.New("usage: pane analyze symbols <file>")
		}
		table, err := (analysis.Client{}).Symbols(context.Background(), args[1])
		if err != nil {
			return err
		}
		return writeJSON(stdout, table)
	case "deps", "dependencies":
		if len(args) != 2 {
			return errors.New("usage: pane analyze deps <file>")
		}
		graph, err := (analysis.Client{}).Dependencies(context.Background(), args[1])
		if err != nil {
			return err
		}
		return writeJSON(stdout, graph)
	case "index":
		return runAnalyzeIndex(args[1:], stdout)
	case "dependents":
		return runAnalyzeDependents(args[1:], stdout)
	default:
		return errors.New("usage: pane analyze symbols|deps|index|dependents ...")
	}
}

func writeJSON(stdout io.Writer, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "%s\n", encoded)
	return nil
}

func runAnalyzeIndex(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: pane analyze index <path...>")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	files, err := analysisInputFiles(args)
	if err != nil {
		return err
	}
	dbPath, err := databasePath()
	if err != nil {
		return err
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	client := analysis.Client{}
	analysisStore := store.NewAnalysisStore(db)
	indexed := 0
	for _, file := range files {
		table, err := client.Symbols(context.Background(), file)
		if err != nil {
			return err
		}
		graph, err := client.Dependencies(context.Background(), file)
		if err != nil {
			return err
		}
		relFile, err := filepath.Rel(env.WorkspaceRoot, file)
		if err != nil || strings.HasPrefix(relFile, "..") {
			relFile = file
		}
		if err := analysisStore.UpsertFile(context.Background(), store.FileAnalysis{
			WorkspaceRoot: env.WorkspaceRoot,
			File:          filepath.ToSlash(relFile),
			Language:      table.Language,
			Symbols:       storeSymbols(table.Symbols),
			Dependencies:  storeDependencies(graph.Dependencies),
		}); err != nil {
			return err
		}
		indexed++
	}
	_, _ = fmt.Fprintf(stdout, "indexed %d file(s)\n", indexed)
	return nil
}

func runAnalyzeDependents(args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: pane analyze dependents <target>")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	dbPath, err := databasePath()
	if err != nil {
		return err
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	dependents, err := store.NewAnalysisStore(db).Dependents(context.Background(), env.WorkspaceRoot, args[0], args[0])
	if err != nil {
		return err
	}
	for _, dep := range dependents {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\t%.2f\n", dep.SourceFile, dep.Target, dep.TargetSymbol, dep.Confidence)
	}
	return nil
}

func analysisInputFiles(paths []string) ([]string, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if isSupportedAnalysisFile(path) {
				abs, err := filepath.Abs(path)
				if err != nil {
					return nil, err
				}
				files = append(files, abs)
			}
			continue
		}
		if err := filepath.WalkDir(path, func(child string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() && shouldSkipAnalysisDir(entry.Name()) {
				return filepath.SkipDir
			}
			if !entry.IsDir() && isSupportedAnalysisFile(child) {
				abs, err := filepath.Abs(child)
				if err != nil {
					return err
				}
				files = append(files, abs)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func shouldSkipAnalysisDir(name string) bool {
	return name == ".git" || name == "node_modules" || name == "vendor" || name == "target" || name == "bin"
}

func isSupportedAnalysisFile(path string) bool {
	switch filepath.Ext(path) {
	case ".go", ".py", ".rs", ".ts", ".tsx":
		return true
	default:
		return false
	}
}

func storeSymbols(symbols []analysis.Symbol) []store.AnalysisSymbol {
	out := make([]store.AnalysisSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		out = append(out, store.AnalysisSymbol{Name: symbol.Name, Kind: symbol.Kind, StartLine: symbol.StartLine, EndLine: symbol.EndLine})
	}
	return out
}

func storeDependencies(dependencies []analysis.Dependency) []store.DependencyEdge {
	out := make([]store.DependencyEdge, 0, len(dependencies))
	for _, dep := range dependencies {
		out = append(out, store.DependencyEdge{Target: dep.Target, TargetSymbol: dep.TargetSymbol, Kind: dep.Kind, Confidence: dep.Confidence})
	}
	return out
}

func runState(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: pane state set|get|list|namespaces|delete ...")
	}
	switch args[0] {
	case "set":
		return runStateSet(args[1:], stdout)
	case "get":
		return runStateGet(args[1:], stdout)
	case "list":
		return runStateList(args[1:], stdout)
	case "namespaces":
		return runStateNamespaces(args[1:], stdout)
	case "delete":
		return runStateDelete(args[1:], stdout)
	default:
		return errors.New("usage: pane state set|get|list|namespaces|delete ...")
	}
}

type stateArgs struct {
	Scope string
	Rest  []string
}

func parseStateScope(args []string) stateArgs {
	parsed := stateArgs{Rest: make([]string, 0, len(args))}
	for _, arg := range args {
		if arg == "--global" {
			parsed.Scope = "global"
			continue
		}
		parsed.Rest = append(parsed.Rest, arg)
	}
	return parsed
}

func runStateSet(args []string, stdout io.Writer) error {
	parsed := parseStateScope(args)
	if len(parsed.Rest) != 2 {
		return errors.New("usage: pane state set [--global] <namespace.key> <json>")
	}
	if !json.Valid([]byte(parsed.Rest[1])) {
		return errors.New("state value must be valid JSON")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestStateSet, Payload: protocol.StateRequestPayloadWithScope(env, parsed.Rest[0], parsed.Rest[1], "", parsed.Scope)})
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
	parsed := parseStateScope(args)
	if len(parsed.Rest) != 1 {
		return errors.New("usage: pane state get [--global] <key>")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestStateGet, Payload: protocol.StateRequestPayloadWithScope(env, parsed.Rest[0], "", "", parsed.Scope)})
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
	parsed := parseStateScope(args)
	if len(parsed.Rest) > 1 {
		return errors.New("usage: pane state list [--global] [prefix]")
	}
	prefix := ""
	if len(parsed.Rest) == 1 {
		prefix = parsed.Rest[0]
	}
	response, err := stateListResponse(prefix, parsed.Scope)
	if err != nil {
		return err
	}
	for _, item := range payloadMaps(response, "items") {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\t%s\n", mapString(item, "key"), mapString(item, "session_id"), mapString(item, "value_json"))
	}
	return nil
}

func runStateNamespaces(args []string, stdout io.Writer) error {
	parsed := parseStateScope(args)
	if len(parsed.Rest) != 0 {
		return errors.New("usage: pane state namespaces [--global]")
	}
	response, err := stateListResponse("", parsed.Scope)
	if err != nil {
		return err
	}
	for _, line := range namespaceLines(payloadMaps(response, "items")) {
		_, _ = fmt.Fprintf(stdout, "%s\t%d\t%s\n", line.Namespace, line.Count, line.SessionID)
	}
	return nil
}

func stateListResponse(prefix, scope string) (protocol.Response, error) {
	env, err := session.DetectEnvironment()
	if err != nil {
		return protocol.Response{}, err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestStateList, Payload: protocol.StateRequestPayloadWithScope(env, "", "", prefix, scope)})
	if err != nil {
		return protocol.Response{}, err
	}
	if !response.OK {
		return protocol.Response{}, errors.New(response.Error)
	}
	return response, nil
}

type namespaceLine struct {
	Namespace string
	Count     int
	SessionID string
	UpdatedAt int64
}

func namespaceLines(items []map[string]any) []namespaceLine {
	byNamespace := make(map[string]namespaceLine)
	for _, item := range items {
		namespace := stateNamespace(mapString(item, "key"))
		line := byNamespace[namespace]
		line.Namespace = namespace
		line.Count++
		updatedAt := mapInt64(item, "updated_at")
		if updatedAt >= line.UpdatedAt {
			line.UpdatedAt = updatedAt
			line.SessionID = mapString(item, "session_id")
		}
		byNamespace[namespace] = line
	}
	namespaces := make([]string, 0, len(byNamespace))
	for namespace := range byNamespace {
		namespaces = append(namespaces, namespace)
	}
	sort.Strings(namespaces)
	lines := make([]namespaceLine, 0, len(namespaces))
	for _, namespace := range namespaces {
		lines = append(lines, byNamespace[namespace])
	}
	return lines
}

func stateNamespace(key string) string {
	if idx := strings.Index(key, "."); idx > 0 {
		return key[:idx]
	}
	return key
}

func runStateDelete(args []string, stdout io.Writer) error {
	parsed := parseStateScope(args)
	if len(parsed.Rest) != 1 {
		return errors.New("usage: pane state delete [--global] <key>")
	}
	env, err := session.DetectEnvironment()
	if err != nil {
		return err
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestStateDelete, Payload: protocol.StateRequestPayloadWithScope(env, parsed.Rest[0], "", "", parsed.Scope)})
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
		return errors.New("usage: pane ask <name-or-id> <message>")
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
