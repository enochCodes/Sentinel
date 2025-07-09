# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOINSTALL=$(GOCMD) install
GOPATH_BIN=$(shell $(GOCMD) env GOPATH)/bin
GOBIN=$(shell $(GOCMD) env GOBIN)
MODULE_PATH=sentinelgo/cmd/sentinelgo

# Output parameters
BINARY_NAME=sentinelgo
BUILD_DIR=build
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS = -ldflags="-X main.version=$(VERSION)"

# Target for GOBIN or GOPATH/bin
TARGET_INSTALL_PATH := $(GOBIN)
ifeq ($(TARGET_INSTALL_PATH),)
TARGET_INSTALL_PATH := $(GOPATH_BIN)
endif

.PHONY: all build install test clean help lint cross-compile

all: build

help:
	@echo "Makefile for sentinelgo"
	@echo ""
	@echo "Usage:"
	@echo "  make build           Build the application for the current OS/Arch into build/"
	@echo "  make install         Install the application using go install"
	@echo "  make test            Run unit tests"
	@echo "  make clean           Remove build artifacts"
	@echo "  make cross-compile   Build for Linux, Windows, and Darwin (amd64)"
	@echo "  make lint            (Placeholder) Run linters"
	@echo ""

build:
	@echo "Building $(BINARY_NAME) version $(VERSION)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MODULE_PATH)/main.go

install:
	@echo "Installing $(BINARY_NAME) to $(TARGET_INSTALL_PATH)..."
	$(GOINSTALL) $(LDFLAGS) $(MODULE_PATH)

test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

clean:
	@echo "Cleaning up build artifacts..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Cross-compilation targets
# These are basic examples. Real .deb or MSI packaging is more complex.
cross-compile: build-linux build-windows build-mac
	@echo "Cross-compilation finished. Binaries in $(BUILD_DIR)/"

build-linux:
	@echo "Building for Linux (amd64)..."
	@mkdir -p $(BUILD_DIR)/linux_amd64
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/linux_amd64/$(BINARY_NAME) $(MODULE_PATH)/main.go
	# Placeholder for .deb: dpkg-deb --build ...

build-windows:
	@echo "Building for Windows (amd64)..."
	@mkdir -p $(BUILD_DIR)/windows_amd64
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/windows_amd64/$(BINARY_NAME).exe $(MODULE_PATH)/main.go
	# Placeholder for .msi or .zip: ...

build-mac:
	@echo "Building for Darwin/Mac (amd64)..."
	@mkdir -p $(BUILD_DIR)/darwin_amd64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/darwin_amd64/$(BINARY_NAME) $(MODULE_PATH)/main.go
	# Placeholder for .dmg or .pkg: ...

# Placeholder for future linting
lint:
	@echo "Linting (placeholder)..."
	# Example: golangci-lint run ./...
	@echo "No linter configured yet."

# Ensure that version is available in main package
# Add this to your sentinelgo/cmd/sentinelgo/main.go:
#
# package main
#
# import (
#   // ... other imports
#   "fmt"
# )
#
# var version = "dev" // Default version, will be overridden by LDFLAGS
#
# func main() {
#   fmt.Printf("SentinelGo version %s\n", version) // Example usage
#   // ... rest of main
# }
#
# Note: The actual main.go already has other content. This is just to show where `var version` goes.
# The `fmt.Printf` for version is optional, but `var version` is needed for ldflags.
