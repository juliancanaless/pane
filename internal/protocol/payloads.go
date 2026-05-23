package protocol

import "github.com/juliancanalez/pane/internal/session"

type SessionEnvironmentPayload struct {
	PaneID          string `json:"pane_id"`
	TTY             string `json:"tty"`
	WorkspaceRoot   string `json:"workspace_root"`
	CWD             string `json:"cwd"`
	Branch          string `json:"branch"`
	RepoID          string `json:"repo_id"`
	GitCommonDir    string `json:"git_common_dir"`
	ParentSessionID string `json:"parent_session_id"`
}

type SessionIntentPayload struct {
	PaneID        string `json:"pane_id"`
	WorkspaceRoot string `json:"workspace_root"`
	Intent        string `json:"intent"`
}

type BoardPayload struct {
	WorkspaceRoot string `json:"workspace_root"`
	RepoID        string `json:"repo_id"`
	Scope         string `json:"scope"`
}

type MessageSendPayload struct {
	PaneID        string `json:"pane_id"`
	WorkspaceRoot string `json:"workspace_root"`
	ToSession     string `json:"to_session"`
	Body          string `json:"body"`
}

type MessageReplyPayload struct {
	PaneID        string `json:"pane_id"`
	WorkspaceRoot string `json:"workspace_root"`
	MessageID     string `json:"message_id"`
	Body          string `json:"body"`
}

type GitPayload struct {
	PaneID        string   `json:"pane_id"`
	WorkspaceRoot string   `json:"workspace_root"`
	Branch        string   `json:"branch"`
	RepoID        string   `json:"repo_id"`
	Args          []string `json:"args"`
	Result        string   `json:"result,omitempty"`
}

func ContinuePayload(value session.Environment, parentSessionID string) map[string]any {
	payload := EnvironmentPayload(value)
	payload["parent_session_id"] = parentSessionID
	return payload
}

func EnvironmentPayload(value session.Environment) map[string]any {
	payload := map[string]any{
		"pane_id":        value.PaneID,
		"tty":            value.TTY,
		"workspace_root": value.WorkspaceRoot,
		"cwd":            value.CWD,
		"branch":         value.Branch,
		"repo_id":        value.RepoID,
		"git_common_dir": value.GitCommonDir,
	}
	if value.ParentSessionID != "" {
		payload["parent_session_id"] = value.ParentSessionID
	}
	return payload
}

func IntentPayload(value session.Environment, intent string) map[string]any {
	return map[string]any{
		"pane_id":        value.PaneID,
		"workspace_root": value.WorkspaceRoot,
		"intent":         intent,
	}
}

func NamePayload(value session.Environment, name string) map[string]any {
	return map[string]any{
		"pane_id":        value.PaneID,
		"workspace_root": value.WorkspaceRoot,
		"name":           name,
	}
}

func BoardRequestPayload(value session.Environment) map[string]any {
	return map[string]any{"workspace_root": value.WorkspaceRoot, "repo_id": value.RepoID}
}

func HistoryRequestPayload(value session.Environment, since int64) map[string]any {
	payload := BoardRequestPayload(value)
	if since > 0 {
		payload["since"] = since
	}
	return payload
}

func StateRequestPayload(value session.Environment, key, jsonValue, prefix string) map[string]any {
	payload := EnvironmentPayload(value)
	payload["key"] = key
	payload["value_json"] = jsonValue
	payload["prefix"] = prefix
	return payload
}

func MessageSendRequestPayload(value session.Environment, toSession, body string) map[string]any {
	return map[string]any{
		"pane_id":        value.PaneID,
		"workspace_root": value.WorkspaceRoot,
		"to_session":     toSession,
		"body":           body,
	}
}

func MessageReplyRequestPayload(value session.Environment, messageID, body string) map[string]any {
	return map[string]any{
		"pane_id":        value.PaneID,
		"workspace_root": value.WorkspaceRoot,
		"message_id":     messageID,
		"body":           body,
	}
}

func GitRequestPayload(value session.Environment, args []string) map[string]any {
	return map[string]any{
		"pane_id":        value.PaneID,
		"workspace_root": value.WorkspaceRoot,
		"branch":         value.Branch,
		"repo_id":        value.RepoID,
		"args":           args,
	}
}

func GitRecordPayload(value session.Environment, args []string, result string) map[string]any {
	payload := GitRequestPayload(value, args)
	payload["result"] = result
	return payload
}
