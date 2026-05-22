.PHONY: build test install clean analysis-build analysis-test

BIN := bin/pane
ANALYZER_BIN := bin/pane-analyze
INSTALL_DIR := $(HOME)/.pane/bin

build: analysis-build
	go build -o $(BIN) ./cmd/pane

analysis-build:
	cargo build --manifest-path analysis/Cargo.toml
	mkdir -p bin
	cp analysis/target/debug/pane-analyze $(ANALYZER_BIN)

test: analysis-test
	go test ./...

analysis-test:
	cargo test --manifest-path analysis/Cargo.toml

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BIN) $(INSTALL_DIR)/pane

clean:
	rm -rf bin analysis/target
