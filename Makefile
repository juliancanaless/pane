.PHONY: build test install install-cli clean restart restart-cli analysis-build analysis-test

BIN := bin/pane
ANALYZER_BIN := bin/pane-analyze
INSTALL_DIR := $(HOME)/.pane/bin

# resign re-signs a binary ad-hoc on macOS. Copying a binary that carries Go's
# linker ad-hoc signature invalidates it on Apple Silicon, and the kernel then
# SIGKILLs the copy; re-signing makes the installed copy runnable. No-op
# elsewhere.
define resign
	@if [ "$$(uname)" = "Darwin" ] && command -v codesign >/dev/null 2>&1; then \
		codesign --force --sign - "$(1)"; \
	fi
endef

build: analysis-build
	go build -o $(BIN) ./cmd/pane

analysis-build:
	. "$$HOME/.cargo/env" 2>/dev/null || true; cargo build --manifest-path analysis/Cargo.toml
	mkdir -p bin
	cp analysis/target/debug/pane-analyze $(ANALYZER_BIN)

test: analysis-test
	go test ./...

analysis-test:
	. "$$HOME/.cargo/env" 2>/dev/null || true; cargo test --manifest-path analysis/Cargo.toml

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BIN) $(INSTALL_DIR)/pane
	$(call resign,$(INSTALL_DIR)/pane)
	cp $(ANALYZER_BIN) $(INSTALL_DIR)/pane-analyze
	$(call resign,$(INSTALL_DIR)/pane-analyze)

# install-cli rebuilds and installs only the Go binary, reusing the already
# installed analyzer. Use this to ship changes that do not touch the Rust
# analyzer without needing a Cargo toolchain.
install-cli:
	go build -o $(BIN) ./cmd/pane
	mkdir -p $(INSTALL_DIR)
	cp $(BIN) $(INSTALL_DIR)/pane
	$(call resign,$(INSTALL_DIR)/pane)

restart:
	-$(BIN) daemon stop
	$(MAKE) build
	$(BIN) daemon start --background

# restart-cli rebuilds only the Go binary, reinstalls it, and restarts the
# daemon. The fast local update path for CLI/daemon changes.
restart-cli:
	-$(INSTALL_DIR)/pane daemon stop
	$(MAKE) install-cli
	$(INSTALL_DIR)/pane daemon start --background

clean:
	rm -rf bin analysis/target
