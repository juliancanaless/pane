package protocol

import "github.com/juliancanalez/pane/internal/session"

type SessionEnvironmentPayload struct {
	PaneID        string `json:"pane_id"`
	TTY           string `json:"tty"`
	WorkspaceRoot string `json:"workspace_root"`
	CWD           string `json:"cwd"`
	Branch        string `json:"branch"`
}

type SessionIntentPayload struct {
	PaneID        string `json:"pane_id"`
	WorkspaceRoot string `json:"workspace_root"`
	Intent        string `json:"intent"`
}

type BoardPayload struct {
	WorkspaceRoot string `json:"workspace_root"`
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

func EnvironmentPayload(value session.Environment) map[string]any {
	return map[string]any{
		"pane_id":        value.PaneID,
		"tty":            value.TTY,
		"workspace_root": value.WorkspaceRoot,
		"cwd":            value.CWD,
		"branch":         value.Branch,
	}
}

func IntentPayload(value session.Environment, intent string) map[string]any {
	return map[string]any{
		"pane_id":        value.PaneID,
		"workspace_root": value.WorkspaceRoot,
		"intent":         intent,
	}
}

func BoardRequestPayload(value session.Environment) map[string]any {
	return map[string]any{"workspace_root": value.WorkspaceRoot}
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
