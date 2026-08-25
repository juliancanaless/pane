# Pane FAQ

## Can agents learn how to use Pane from only the installed binary?

Yes. Use the built-in docs:

```bash
pane docs
pane docs quickstart
pane docs agents
pane docs faq
pane docs links
```

These docs are compiled into the `pane` binary so agents can discover the basic operating contract without needing a checked-out source repo.

## Do agents have to use Pane themselves, or does the human update it?

Agents are expected to use Pane themselves.

The human can inspect the board, but Pane is designed so the human is not the message bus. A Pane-aware agent should initialize its session, read the board/summary, set intent, check inbox, coordinate with peers, and leave handoff state as part of its normal loop.

See [`AGENTS.md`](AGENTS.md) and [`docs/for-agents.md`](docs/for-agents.md).

## What if I start a new agent in the same terminal pane but do not want it to inherit context?

Today, close the current Pane session first:

```bash
pane close
pane init
pane intent "new independent task"
```

Pane resumes active/idle sessions by terminal pane identity. Closing the previous session makes the next `pane init` create a fresh independent session with no parent link and no inherited intent.

Advanced escape hatch:

```bash
PANE_PANE_ID="manual:new-task" pane init
```

That forces a distinct Pane identity, but `pane close && pane init` is the normal workflow.

A future release may add a first-class `pane new` command for this.

## What if I want a new agent to inherit context from a previous session?

Use:

```bash
pane continue <session-id-or-short-id>
pane summary
```

This links the new/current pane session to the prior session and carries continuity context into summaries/history.

## How does Pane identify a session?

Pane sessions are tied to terminal pane identity, not the agent process.

Identity priority:

1. `PANE_PANE_ID` override
2. Zellij session + pane ID
3. tmux pane ID
4. cmux surface ID (only when no inner multiplexer is running in the surface)
5. TTY fallback

This lets a replacement agent in the same pane resume the pane's context unless the old session was closed.

Claude Code sessions get a second identity: with the integration installed
(`pane setup --claude`), the pane session is also bound to the Claude session
id, so Pane's hooks and statusline find the right session even from
subprocesses that have no terminal of their own.

## Is Pane an orchestrator?

No.

Pane does not assign tasks, schedule agents, or control providers. It is local shared memory and coordination. Agents remain autonomous; Pane gives them a common local coordination surface.

## Does Pane work only with a specific AI provider?

No.

Pane is provider-agnostic. If an agent can run shell commands, it can participate. It works with terminal-based workflows such as Claude Code, Codex, Cursor-style agents, Pi, Aider, custom scripts, and human shells.

## How do I install Pane?

Recommended on macOS:

```bash
brew tap juliancanaless/pane
brew install pane
pane setup
pane doctor
```

Homebrew binary assets exist for Intel and Apple Silicon Macs.

From source:

```bash
git clone git@github.com:juliancanaless/pane.git
cd pane
make build
./bin/pane setup
./bin/pane doctor
```

## What does `pane setup` do?

`pane setup` installs local runtime files and starts Pane:

- installs `pane` and `pane-analyze` under `~/.pane/bin`
- installs a transparent git shim under `~/.pane/shims`
- appends a managed shell hook to `.zshrc` or `.bashrc`
- starts the daemon in the background

Safer options:

```bash
pane setup --no-shell
pane setup --no-shim
pane setup --no-daemon
pane setup --print-shell
```

With `--claude` it also wires Claude Code hooks and a statusline into
`~/.claude/settings.json` (additive and idempotent — existing hooks and a
custom statusline are preserved).

## What does `pane doctor` check?

`pane doctor` reports platform identity, installed binary paths, analyzer availability, database/socket/PID/log paths, git shim, shell hook, and daemon health.

## Where does Pane store data?

By default under `~/.pane`:

- SQLite DB: `~/.pane/pane.db`
- socket: `~/.pane/pane.sock`
- PID file: `~/.pane/pane.pid`
- logs: `~/.pane/logs/pane.log`

You can override paths with:

```bash
PANE_DB_PATH=...
PANE_SOCKET_PATH=...
PANE_PID_PATH=...
PANE_LOG_PATH=...
```

## Will Pane's SQLite database grow forever?

Pane currently persists sessions, messages, file activity, git events, agent state, and analysis data locally.

Views already reduce noise with stale-session hiding and activity decay, but v0.1.1 does not yet aggressively prune or compact old SQLite rows automatically.

In normal early use the database should stay small. For long-running/high-volume usage, explicit retention controls such as `pane vacuum`, configurable pruning windows, or automatic compaction are likely future hardening work.

## How do I remove stale sessions from the board?

Default board output hides closed and first-pass stale sessions. To explicitly close stale active/idle sessions in the current workspace:

```bash
pane sessions prune
pane board
```

To close the current session:

```bash
pane close
```

## How do agents coordinate without the human relaying messages?

Use Pane messaging:

```bash
pane ask <session-id-or-name> "Are you still touching auth/session.ts?"
pane inbox
pane reply <message-id> "Done with that file."
```

Agents should use this for routine coordination instead of asking the human to copy messages between panes.

## How does Pane work with Git worktrees?

Pane keeps worktree-local context for cwd, file watching, and activity. It also detects shared repository identity from Git common-dir metadata.

Use repo scope to see related worktrees:

```bash
pane board --repo
pane history --repo
pane history --repo --lineage
```

## Does Pane require a git repository?

No. Since v0.1.5, Pane works in any directory. Outside a git repository the
workspace root falls back to the current directory (or `PANE_WORKSPACE_ROOT`
if set), and `pane init`, `board`, `ask`/`inbox`, `intent`, and `state` all
work normally. What degrades: there is no branch, git guardrails have nothing
to guard, and `--repo` scope is unavailable.

One deliberate asymmetry: outside git repositories the shell hook's automatic
heartbeat only refreshes sessions that already exist — it never creates one.
Only an explicit `pane init` creates a session there, so `cd`-ing through
random directories never pollutes the board. This is designed for e.g. a
manager agent coordinating from a scratch directory that is not a checkout.

## How do I upgrade Pane, and does the daemon pick up the new version?

```bash
brew upgrade pane
pane setup   # refreshes ~/.pane/bin and the git shim
```

The daemon updates itself: every daemon response carries its version, and any
`pane` command that reaches an older daemon restarts it in place with the
current binary (a `[Pane] daemon ... restarting` notice goes to stderr). A
newer daemon is never downgraded by an older CLI. `pane doctor` and
`pane daemon status` show both CLI and daemon versions.

## The daemon disappears while a coding agent is working. Do I have to restart it?

No. Agents kill the process group of a finished or timed-out tool call, which
takes a daemon started inside one with it. Every `pane` command now starts a
detached daemon when it finds none and retries the request once, so the next
command heals it. Attempts are rate-limited to one per 10 seconds, and the
daemon logs its version and pid on every start so `~/.pane/logs/pane.log` shows how
often it is being replaced.

```bash
PANE_NO_AUTOSTART=1 pane board      # never auto-start
pane daemon start --foreground      # blocking, for debugging or a service manager
```

On macOS, `pane setup` also registers the daemon as a launchd agent
(`~/Library/LaunchAgents/com.pane.daemon.plist`), so launchd respawns it
within seconds of a crash or kill without waiting for the next pane command.
A clean `pane daemon stop` stays stopped. Daemon log lines are timestamped,
so `~/.pane/logs/pane.log` shows exactly when each start and shutdown
happened. To remove the agent: `launchctl bootout gui/$(id -u)/com.pane.daemon`
and delete the plist.

## Does Pane replace Git?

No.

Pane wraps git only to provide shared-state preflight warnings and event recording:

```bash
pane git status
pane git rebase main
pane git push --force-with-lease
```

Git remains Git.

## What is `pane-analyze`?

`pane-analyze` is the Rust/tree-sitter helper used for symbols and dependency edges. `pane setup` and Homebrew install it alongside `pane`.

If semantic commands fail, run:

```bash
pane doctor
```

or set:

```bash
PANE_ANALYZER_PATH=/path/to/pane-analyze
```

## What does semantic overlap mean today?

In v0.1.1, semantic overlap is first-pass dependency awareness based mostly on imports/use/require edges and persisted symbol tables.

It is not yet full signature-aware callsite analysis. Treat it as better-than-file-proximity warning, not perfect compiler-level impact analysis.

## Can I use Pane without the git shim?

Yes.

You can always run:

```bash
pane git <args...>
```

The git shim only makes normal `git` commands route through Pane automatically.

If you do not want the shim:

```bash
pane setup --no-shim
```

## Can I use Pane without modifying my shell rc file?

Yes.

```bash
pane setup --no-shell
pane setup --print-shell
```

Then manually add the printed hook if you want it.

## Is Pane local or cloud-based?

Pane is local-first. There is no remote service. State is stored locally in SQLite and accessed through a local Unix socket daemon.

## How does Pane integrate with Claude Code?

Run `pane setup --claude` once. It installs:

- a **SessionStart hook** — registers/resumes the pane session and injects its
  identity into the agent's context on startup, resume, `/clear`, and after
  every context compaction. Compaction no longer loses or tangles pane sessions.
- a **statusline** — the bottom of the Claude Code UI shows the pane session
  name, current intent, and unread message count at all times.
- a **Stop hook** — if pane messages arrived while the agent worked, they are
  delivered before it goes idle so it replies first.
- a **UserPromptSubmit hook** — waiting messages are surfaced at the start of
  each new turn.

## Do incoming pane messages wake an idle Claude Code session?

Inside a multiplexer pane — cmux, zellij, or tmux — yes: the daemon nudges
the recipient's pane by typing `pane inbox` into it (disable with
`PANE_WAKE=off`). In a plain terminal there is no supported way to inject
input into an idle interactive Claude session, so messages surface at the
next turn instead — and an agent that is mid-task is always stopped from
going idle until it replies. The statusline unread counter updates
regardless.

## How do I uninstall Pane?

If installed through Homebrew:

```bash
brew uninstall pane
brew untap juliancanaless/pane
```

Optional local state cleanup:

```bash
rm -rf ~/.pane
```

Be careful: removing `~/.pane` deletes Pane's local history/state.
