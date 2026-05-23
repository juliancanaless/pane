# Install Pane

Pane is currently distributed as a source-built local tool. The first public release is `v0.1.0`.

## Option 1: Homebrew formula file

```bash
curl -L -o pane.rb https://raw.githubusercontent.com/juliancanaless/pane/main/packaging/homebrew/pane.rb
brew install ./pane.rb
pane setup
pane doctor
```

For the latest unreleased build from `main`:

```bash
curl -L -o pane.rb https://raw.githubusercontent.com/juliancanaless/pane/main/packaging/homebrew/pane.rb
brew install --HEAD ./pane.rb
pane setup
pane doctor
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
