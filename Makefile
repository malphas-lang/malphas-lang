# Malphas Makefile

BINARY_NAME=malphas
GO=go
# Default install directory, can be overridden: make install INSTALL_DIR=/usr/local/bin
INSTALL_DIR?=$(HOME)/.local/bin

.PHONY: all build install clean test

all: build

build:
	$(GO) build -o $(BINARY_NAME) ./cmd/malphas

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY_NAME) $(INSTALL_DIR)
	chmod +x $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed $(BINARY_NAME) to $(INSTALL_DIR)"

clean:
	rm -f $(BINARY_NAME)

test:
	$(GO) test ./...
