package cli

import (
	"errors"
	"fmt"
	"io"
	"sort"
)

const docsIndex = `Pane built-in docs

Pane is shared local memory for AI coding agents. Agents are expected to use Pane themselves so the human does not have to relay context or keep state updated.

Available docs:
  pane docs quickstart   Minimal first-run workflow
  pane docs agents       Operating contract for AI coding agents
  pane docs commands     Common command reference
  pane docs faq          Common edge cases and answers
  pane docs links        Full online documentation links
`

const docsQuickstart = `Pane quickstart

Install:
  brew tap juliancanaless/pane
  brew install pane
  pane setup
  pane doctor

Start in a workspace:
  pane init
  pane heartbeat
  pane summary
  pane board
  pane inbox
  pane intent "what I am doing now"

Before risky work:
  pane board --repo
  pane git status

Coordinate with peers:
  pane ask <session-id-or-name> "question"
  pane inbox
  pane reply <message-id> "answer"

Coordinate across workspaces:
  pane board --global                       See every session on the machine
  pane ask --global <full-session-id> "question"   Message a session in another workspace

Leave handoff state:
  pane state set agent.handoff '{"summary":"what changed","next":"what to do next"}'
  pane history --since 24h --lineage
  pane close
`

const docsAgents = `Pane agent operating contract

If you are an AI coding agent, your job is to keep Pane useful while you work. The human can inspect Pane, but should not have to maintain it for you.

Expected loop:
  1. Run pane init when starting in a workspace.
  2. Read pane summary, pane board, and pane inbox before acting.
  3. Set pane intent before meaningful changes.
  4. Update intent whenever you switch tasks.
  5. Use pane ask / pane reply for routine coordination with other sessions.
  6. Use pane git for risky git operations.
  7. Store compact handoff/memory facts with pane state when useful.
  8. Run pane close when your pane's work is done.

Do not assume the human knows what other panes are doing. Inspect Pane yourself.
Do not ask the human to relay routine cross-agent messages. Use Pane messaging.
To coordinate with an agent in another repository or workspace, run pane board --global
to find its full session id, then pane ask --global <full-session-id> "<message>".

Useful state conventions:
  pane state set agent.notes '{"handoff":"tests need review"}'
  pane state set summary.note '{"text":"shows in pane summary"}'
  pane state namespaces
  pane state list agent.

Non-git directories:
Pane does not require a git repository. In a plain directory (for example a
manager agent coordinating from a scratch dir) pane init, board, ask/inbox,
intent, and state work normally; the workspace root is the current directory
(or PANE_WORKSPACE_ROOT). There is no branch, and git guardrails and --repo
scope are off. Sessions there are only created by an explicit pane init,
never by shell-hook heartbeats.

Claude Code sessions:
If Pane's Claude Code integration is installed (pane setup --claude), your
pane session is registered automatically at startup and re-announced after
compaction — when you see a [Pane] identity block in your context, that
session is already yours; do not run pane init again. Unread pane messages
are delivered to you before you go idle; reply with pane reply <message-id>.
`

const docsCommands = `Pane common commands

Session lifecycle:
  pane init                         Register/resume this terminal pane
  pane heartbeat                    Refresh cwd/branch/last-seen
  pane intent <text>                Set current task
  pane close                        Close current session
  pane continue <session-id>        Link to prior session context

Awareness:
  pane summary                      Startup/resume context
  pane board                        Active workspace board
  pane board --repo                 Board across sibling worktrees
  pane board --global               Board across every workspace on the machine
  pane history --lineage            Show session lineage
  pane history --format work-log    Work report

Coordination:
  pane ask <target> <message>       Ask another session
  pane ask --global <full-id> <msg> Ask a session in another workspace (by full ID)
  pane inbox                        Read queued messages
  pane reply <msg-id> <message>     Reply to thread

Agent memory:
  pane state set <key> <json>
  pane state get <key>
  pane state list [prefix]
  pane state namespaces
  pane state set --global <key> <json>

Git guardrails:
  pane git <git-args...>

Workers:
  pane spawn <command> [args...]

Setup/diagnostics:
  pane setup
  pane setup --claude
  pane setup --no-shell --no-daemon
  pane doctor
  pane docs [quickstart|agents|commands|faq|links]

Claude Code integration (installed by pane setup --claude):
  pane hook session-start           SessionStart hook: re-inject pane identity (survives compaction)
  pane hook stop                    Stop hook: deliver unread messages before the agent goes idle
  pane hook user-prompt-submit      Surface unread count at the start of each turn
  pane statusline                   One-line session/intent/unread status for the Claude UI
`

const docsFAQ = `Pane FAQ

Q: What if I start a new agent in the same pane but do not want inherited context?
A: Close the current session first:
     pane close
     pane init
     pane intent "new independent task"

Q: What if I want to inherit prior context?
A: Use:
     pane continue <session-id-or-short-id>
     pane summary

Q: Is Pane an orchestrator?
A: No. Pane does not assign tasks or control agents. It is local shared memory and coordination.

Q: Does Pane work only with one AI provider?
A: No. If an agent can run shell commands, it can participate.

Q: Where is data stored?
A: SQLite and runtime files live under ~/.pane by default. Override with PANE_DB_PATH, PANE_SOCKET_PATH, PANE_PID_PATH, and PANE_LOG_PATH.

Q: Will SQLite grow forever?
A: v0.1.x hides/summarizes older activity in views but does not aggressively prune old rows yet. Retention controls are future hardening.

Q: Can I avoid shell rc edits?
A: Yes:
     pane setup --no-shell
     pane setup --print-shell

Q: Can I avoid the git shim?
A: Yes:
     pane setup --no-shim
     pane git <args...>

Q: How do worktrees work?
A: Use workspace-local commands by default and --repo to aggregate sibling worktrees:
     pane board --repo
     pane history --repo --lineage

Q: Does Pane require a git repository?
A: No. Outside a repository the workspace root is the current directory (or
   PANE_WORKSPACE_ROOT); branch, git guardrails, and --repo scope are off.
   Only an explicit pane init creates a session there.

Q: How do upgrades reach the background daemon?
A: Automatically. Any pane command that reaches a daemon older than the CLI
   restarts it with the current binary. After brew upgrade pane, run
   pane setup to refresh the ~/.pane/bin copies.

Q: How does Pane integrate with Claude Code?
A: Run pane setup --claude once. It wires Claude Code user settings with a
   SessionStart hook (re-injects the pane session identity after startup,
   resume, and every compaction), a Stop hook (delivers unread pane messages
   so the agent replies before going idle), a UserPromptSubmit hook (surfaces
   the unread count each turn), and a statusline showing session, intent,
   and unread messages at the bottom of the Claude UI.

Q: Do incoming messages wake an idle Claude session?
A: Only in tmux panes, where the daemon nudges the pane by typing pane inbox
   (disable with PANE_WAKE=off). Elsewhere an idle agent sees messages at
   its next turn; a busy agent is always stopped from going idle until it
   replies, and the statusline always shows the unread count.
`

const docsLinks = `Pane online docs

Repository:
  https://github.com/juliancanaless/pane

Install:
  https://github.com/juliancanaless/pane/blob/main/INSTALL.md

FAQ:
  https://github.com/juliancanaless/pane/blob/main/FAQ.md

Agent guide:
  https://github.com/juliancanaless/pane/blob/main/docs/for-agents.md

Demo transcript:
  https://github.com/juliancanaless/pane/blob/main/docs/demo.md

Architecture:
  https://github.com/juliancanaless/pane/blob/main/ARCHITECTURE.md
`

func runDocs(args []string, stdout io.Writer) error {
	if len(args) > 1 {
		return errors.New("usage: pane docs [quickstart|agents|commands|faq|links]")
	}
	topic := "index"
	if len(args) == 1 {
		topic = args[0]
	}
	docs := map[string]string{
		"index":      docsIndex,
		"quickstart": docsQuickstart,
		"agents":     docsAgents,
		"commands":   docsCommands,
		"faq":        docsFAQ,
		"links":      docsLinks,
	}
	text, ok := docs[topic]
	if !ok {
		topics := make([]string, 0, len(docs)-1)
		for key := range docs {
			if key != "index" {
				topics = append(topics, key)
			}
		}
		sort.Strings(topics)
		return fmt.Errorf("unknown docs topic %q; available: %v", topic, topics)
	}
	_, _ = fmt.Fprint(stdout, text)
	return nil
}
