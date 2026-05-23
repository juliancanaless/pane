# Install Pane

Pane is currently distributed through a Homebrew tap or source build. The latest public release is `v0.1.2`, with Homebrew binary assets for Intel and Apple Silicon Macs.

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

## Verify

```bash
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

Run `pane doctor` to see the paths and health checks.

For common setup and workflow questions, see [`FAQ.md`](FAQ.md).
