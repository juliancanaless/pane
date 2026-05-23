.PHONY: build test install clean restart analysis-build analysis-test

BIN := bin/pane
ANALYZER_BIN := bin/pane-analyze
INSTALL_DIR := $(HOME)/.pane/bin

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

restart:
	-$(BIN) daemon stop
	$(MAKE) build
	$(BIN) daemon start --background

clean:
	rm -rf bin analysis/target
