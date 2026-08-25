# Install Pane

Pane is currently distributed through a Homebrew tap or source build. The latest public release is `v0.1.9`, with Homebrew binary assets for Intel and Apple Silicon Macs.

## Option 1: Homebrew tap

```bash
brew tap juliancanaless/pane
brew install pane
pane setup
pane doctor
pane docs quickstart
```

For the latest unreleased build from `main`:

```bash
brew tap juliancanaless/pane
brew install --HEAD pane
pane setup
pane doctor
pane docs quickstart
```

## Option 2: Build from source

Requirements:

- Go
- Rust/Cargo

```bash
git clone git@github.com:juliancanaless/pane.git
cd pane
make build
./bin/pane setup
./bin/pane doctor
```

If you do not want setup to edit your shell rc file or start the daemon automatically:

```bash
./bin/pane setup --no-shell --no-daemon
./bin/pane setup --print-shell
```

## Claude Code integration

If you use Claude Code, run:

```bash
pane setup --claude
```

This additively wires `~/.claude/settings.json` with a Pane statusline
(session, intent, unread messages at the bottom of the UI) and three hooks:
SessionStart (pane identity survives startup, resume, and compaction), Stop
(unread pane messages are delivered before the agent goes idle), and
UserPromptSubmit (unread count surfaces each turn). Existing hooks and a
custom statusline are left untouched; re-running is safe.

## Upgrade

```bash
brew upgrade pane
pane setup
```

`pane setup` refreshes the `~/.pane/bin` copies that the shell hook and git
shim point at. The background daemon takes care of itself: the first `pane`
command that reaches a daemon older than the CLI restarts it with the new
binary automatically, and any command that finds no daemon at all starts a
detached one and retries. That last part matters under coding agents, which
kill the process group of a finished tool call and take a daemon started
inside one with it. Set `PANE_NO_AUTOSTART=1` to turn it off, or run
`pane daemon start --foreground` to keep a daemon in the terminal for
debugging or under a service manager.

## Verify

```bash
pane version
pane daemon health
pane docs agents
pane init
pane summary
pane history --format work-log
```

## What setup installs

`pane setup` installs:

- `~/.pane/bin/pane`
- `~/.pane/bin/pane-analyze`
- `~/.pane/shims/git`
- a managed shell hook block in `.zshrc` or `.bashrc`
- a background Pane daemon
- with `--claude`: Pane hooks and statusline in `~/.claude/settings.json`

Run `pane doctor` to see the paths and health checks.

For common setup and workflow questions, see [`FAQ.md`](FAQ.md).
