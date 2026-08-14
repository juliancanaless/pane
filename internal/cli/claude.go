package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/juliancanalez/pane/internal/protocol"
	"github.com/juliancanalez/pane/internal/session"
)

// agentHookInput is the JSON Claude Code pipes to hook and statusline
// commands on stdin. Only the fields Pane needs are decoded; unknown fields
// and missing fields are both fine.
type agentHookInput struct {
	SessionID      string `json:"session_id"`
	HookEventName  string `json:"hook_event_name"`
	Source         string `json:"source"`
	CWD            string `json:"cwd"`
	StopHookActive bool   `json:"stop_hook_active"`
	Workspace      struct {
		CurrentDir string `json:"current_dir"`
		ProjectDir string `json:"project_dir"`
	} `json:"workspace"`
}

func readAgentHookInput(stdin io.Reader) agentHookInput {
	var input agentHookInput
	data, err := io.ReadAll(io.LimitReader(stdin, 1<<20))
	if err != nil || len(bytes.TrimSpace(data)) == 0 {
		return input
	}
	_ = json.Unmarshal(data, &input)
	return input
}

// enterAgentDir moves this short-lived process into the agent's working
// directory so workspace detection matches the agent's project rather than
// wherever the host process happened to spawn the hook.
func enterAgentDir(input agentHookInput) {
	for _, dir := range []string{input.CWD, input.Workspace.CurrentDir, input.Workspace.ProjectDir} {
		if dir != "" && os.Chdir(dir) == nil {
			return
		}
	}
}

func agentRequestPayload(input agentHookInput) (map[string]any, error) {
	env, err := session.DetectEnvironment()
	if err != nil {
		return nil, err
	}
	payload := protocol.EnvironmentPayload(env)
	if input.SessionID != "" {
		payload["agent_session_id"] = input.SessionID
	}
	return payload, nil
}

func runHook(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: pane hook <session-start|stop|user-prompt-submit>")
	}
	input := readAgentHookInput(stdin)
	enterAgentDir(input)
	switch args[0] {
	case "session-start":
		return runHookSessionStart(input, stdout)
	case "stop":
		return runHookStop(input, stdout)
	case "user-prompt-submit":
		return runHookUserPromptSubmit(input, stdout)
	default:
		return fmt.Errorf("unknown hook %q; available: session-start, stop, user-prompt-submit", args[0])
	}
}

// runHookSessionStart registers/resumes the pane session and prints a compact
// identity block. Claude Code injects the stdout of SessionStart hooks into
// the agent's context, and the hook fires after every compaction, so the
// agent never loses track of which pane session it is. Failures stay silent:
// a broken Pane install must not break agent startup.
func runHookSessionStart(input agentHookInput, stdout io.Writer) error {
	payload, err := agentRequestPayload(input)
	if err != nil {
		return nil
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestSessionInit, Payload: payload})
	if err != nil || !response.OK {
		return nil
	}
	state := "created"
	if payloadBool(response, "resumed") {
		state = "resumed"
	}
	branch := payloadString(response, "branch")
	if branch == "" {
		branch = "(none)"
	}
	if input.Source == "compact" {
		_, _ = fmt.Fprintf(stdout, "Context was compacted. Your Pane coordination identity below is unchanged — do not run `pane init` again.\n")
	}
	_, _ = fmt.Fprintf(stdout, "[Pane] session %s (%s) · workspace %s · branch %s\n",
		payloadString(response, "session_id"), state, payloadString(response, "workspace_root"), branch)
	if intent := payloadString(response, "intent"); intent != "" {
		_, _ = fmt.Fprintf(stdout, "Current intent: %s\n", intent)
	}
	if unread := unreadCount(input); unread > 0 {
		_, _ = fmt.Fprintf(stdout, "Unread pane messages: %d — read them with `pane inbox` and reply with `pane reply <message-id> \"...\"`.\n", unread)
	}
	_, _ = fmt.Fprintf(stdout, "Keep coordination current yourself: `pane intent \"...\"` on task switches, `pane board` for peers, `pane ask`/`pane reply` to coordinate, `pane close` when finished. Full contract: `pane docs agents`.\n")
	return nil
}

// runHookStop blocks the agent from going idle while pane messages are
// waiting: the messages are delivered inside the block reason, so the agent
// replies before stopping. Delivery marks them read, so the next stop passes.
func runHookStop(input agentHookInput, stdout io.Writer) error {
	if input.StopHookActive {
		return nil
	}
	payload, err := agentRequestPayload(input)
	if err != nil {
		return nil
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestAgentMessages, Payload: payload})
	if err != nil || !response.OK || !payloadBool(response, "found") {
		return nil
	}
	count := mapInt64(response.Payload, "count")
	if count == 0 {
		return nil
	}
	reason := fmt.Sprintf("[Pane] %d unread pane message(s) arrived while you were working:\n\n%s\nHandle them before stopping: reply with `pane reply <message-id> \"...\"` (send a short acknowledgment if a full answer needs more work).",
		count, payloadString(response, "text"))
	return json.NewEncoder(stdout).Encode(map[string]any{"decision": "block", "reason": reason})
}

// runHookUserPromptSubmit surfaces waiting messages at the start of the next
// turn without delivering them, so the stop hook still guarantees a reply.
func runHookUserPromptSubmit(input agentHookInput, stdout io.Writer) error {
	unread := unreadCount(input)
	if unread == 0 {
		return nil
	}
	context := fmt.Sprintf("[Pane] %d unread pane message(s) waiting — run `pane inbox` and reply once the user's request is handled.", unread)
	return json.NewEncoder(stdout).Encode(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     "UserPromptSubmit",
			"additionalContext": context,
		},
	})
}

func unreadCount(input agentHookInput) int64 {
	payload, err := agentRequestPayload(input)
	if err != nil {
		return 0
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestAgentContext, Payload: payload})
	if err != nil || !response.OK || !payloadBool(response, "found") {
		return 0
	}
	return mapInt64(response.Payload, "unread")
}

const (
	ansiDim    = "\x1b[2m"
	ansiYellow = "\x1b[33;1m"
	ansiReset  = "\x1b[0m"
)

// statuslineIntentMax keeps long intents from crowding out the rest of the
// statusline; the full intent is always available via pane status.
const statuslineIntentMax = 40

func truncateStatus(text string, max int) string {
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max-1]) + "…"
}

// runStatusline renders the one-line Pane status Claude Code shows at the
// bottom of the UI: session identity, current intent, and unread messages.
func runStatusline(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) != 0 {
		return errors.New("usage: pane statusline (reads agent status JSON on stdin)")
	}
	input := readAgentHookInput(stdin)
	enterAgentDir(input)
	payload, err := agentRequestPayload(input)
	if err != nil {
		_, _ = fmt.Fprintf(stdout, "%spane: unavailable%s\n", ansiDim, ansiReset)
		return nil
	}
	response, err := sendDaemonRequest(protocol.Request{Type: protocol.RequestAgentContext, Payload: payload})
	if err != nil {
		_, _ = fmt.Fprintf(stdout, "%spane: daemon offline%s\n", ansiDim, ansiReset)
		return nil
	}
	if !response.OK || !payloadBool(response, "found") {
		_, _ = fmt.Fprintf(stdout, "%spane: no session — run `pane init`%s\n", ansiDim, ansiReset)
		return nil
	}
	identity := payloadString(response, "name")
	if identity == "" {
		identity = payloadString(response, "short_id")
	}
	intent := payloadString(response, "intent")
	if intent == "" {
		intent = "no intent set"
	}
	line := fmt.Sprintf("pane %s · %s", identity, truncateStatus(intent, statuslineIntentMax))
	if payloadString(response, "repo_id") == "" {
		line += " · no-git"
	}
	if unread := mapInt64(response.Payload, "unread"); unread > 0 {
		_, _ = fmt.Fprintf(stdout, "%s%s%s · %s✉ %d unread%s\n", ansiDim, line, ansiReset, ansiYellow, unread, ansiReset)
		return nil
	}
	_, _ = fmt.Fprintf(stdout, "%s%s · ✉ 0%s\n", ansiDim, line, ansiReset)
	return nil
}

// installClaudeIntegration wires Pane into Claude Code's user-level
// settings: a statusline plus SessionStart/Stop/UserPromptSubmit hooks. The
// edit is additive and idempotent — existing user configuration is preserved,
// and pane-managed entries are recognized by their command and updated in
// place.
func installClaudeIntegration(stdout io.Writer, paneBin string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		return err
	}
	settings := map[string]any{}
	if data, readErr := os.ReadFile(settingsPath); readErr == nil && len(bytes.TrimSpace(data)) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("cannot parse %s: %w", settingsPath, err)
		}
	}

	statuslineCommand := paneBin + " statusline"
	switch existing := settings["statusLine"].(type) {
	case map[string]any:
		if command := mapString(existing, "command"); strings.Contains(command, "pane statusline") {
			existing["command"] = statuslineCommand
			_, _ = fmt.Fprintln(stdout, "claude statusline: already installed")
		} else {
			_, _ = fmt.Fprintf(stdout, "claude statusline: skipped (custom statusline present; to show Pane in it, call `%s` from your statusline command)\n", statuslineCommand)
		}
	default:
		settings["statusLine"] = map[string]any{"type": "command", "command": statuslineCommand}
		_, _ = fmt.Fprintln(stdout, "claude statusline: installed")
	}

	hooks, _ := settings["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	for _, spec := range []struct {
		event   string
		matcher string
		hook    string
	}{
		{"SessionStart", "startup|resume|clear|compact", "session-start"},
		{"Stop", "", "stop"},
		{"UserPromptSubmit", "", "user-prompt-submit"},
	} {
		command := paneBin + " hook " + spec.hook
		if ensureClaudeHook(hooks, spec.event, spec.matcher, command) {
			_, _ = fmt.Fprintf(stdout, "claude hook %s: installed\n", spec.event)
		} else {
			_, _ = fmt.Fprintf(stdout, "claude hook %s: already installed\n", spec.event)
		}
	}
	settings["hooks"] = hooks

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, append(data, '\n'), 0o644)
}

// ensureClaudeHook adds the pane hook entry for an event unless one is
// already present, refreshing the command path of existing pane entries.
// Returns true when a new entry was added.
func ensureClaudeHook(hooks map[string]any, event, matcher, command string) bool {
	groups, _ := hooks[event].([]any)
	for _, group := range groups {
		groupMap, ok := group.(map[string]any)
		if !ok {
			continue
		}
		entries, _ := groupMap["hooks"].([]any)
		for _, entry := range entries {
			entryMap, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			if strings.Contains(mapString(entryMap, "command"), "pane hook ") {
				entryMap["command"] = command
				if matcher != "" {
					groupMap["matcher"] = matcher
				}
				return false
			}
		}
	}
	group := map[string]any{
		"hooks": []any{map[string]any{"type": "command", "command": command}},
	}
	if matcher != "" {
		group["matcher"] = matcher
	}
	hooks[event] = append(groups, group)
	return true
}
