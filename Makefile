.PHONY: build test install clean

BIN := bin/pane
INSTALL_DIR := $(HOME)/.pane/bin

build:
	go build -o $(BIN) ./cmd/pane

test:
	go test ./...

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BIN) $(INSTALL_DIR)/pane

clean:
	rm -rf bin
