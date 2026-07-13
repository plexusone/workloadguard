.PHONY: build install clean test lint run check validate

# Build variables
BINARY := workloadguard
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X github.com/plexusone/workloadguard/internal/cli.Version=$(VERSION) \
                     -X github.com/plexusone/workloadguard/internal/cli.Commit=$(COMMIT) \
                     -X github.com/plexusone/workloadguard/internal/cli.Date=$(DATE)"

# Installation paths
PREFIX := $(HOME)
BINDIR := $(PREFIX)/bin
CONFIGDIR := $(PREFIX)/.config/workloadguard
LAUNCHAGENTS := $(PREFIX)/Library/LaunchAgents

# Build the binary
build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/workloadguard

# Install binary and config
install: build
	@mkdir -p $(BINDIR) $(CONFIGDIR)
	cp $(BINARY) $(BINDIR)/$(BINARY)
	chmod 755 $(BINDIR)/$(BINARY)
	@if [ ! -f $(CONFIGDIR)/config.toml ]; then \
		cp configs/workloadguard.toml $(CONFIGDIR)/config.toml; \
		echo "Installed config to $(CONFIGDIR)/config.toml"; \
	else \
		echo "Config already exists at $(CONFIGDIR)/config.toml"; \
	fi

# Install launchd plist
install-launchd: install
	@mkdir -p $(LAUNCHAGENTS)
	@sed "s|YOUR_USERNAME|$(USER)|g; s|/Users/YOUR_USERNAME|$(HOME)|g" \
		configs/com.plexusone.workloadguard.plist > $(LAUNCHAGENTS)/com.plexusone.workloadguard.plist
	@echo "Installed launchd plist to $(LAUNCHAGENTS)/com.plexusone.workloadguard.plist"
	@echo ""
	@echo "To start the daemon:"
	@echo "  launchctl bootstrap gui/$$UID $(LAUNCHAGENTS)/com.plexusone.workloadguard.plist"
	@echo ""
	@echo "To stop the daemon:"
	@echo "  launchctl bootout gui/$$UID/com.plexusone.workloadguard"

# Uninstall launchd plist
uninstall-launchd:
	-launchctl bootout gui/$$UID/com.plexusone.workloadguard 2>/dev/null
	rm -f $(LAUNCHAGENTS)/com.plexusone.workloadguard.plist

# Uninstall everything
uninstall: uninstall-launchd
	rm -f $(BINDIR)/$(BINARY)
	@echo "Config left at $(CONFIGDIR)/config.toml (remove manually if desired)"

# Clean build artifacts
clean:
	rm -f $(BINARY)
	go clean

# Run tests
test:
	go test -v ./...

# Run linter
lint:
	golangci-lint run

# Run in dry-run mode
run: build
	./$(BINARY) run --dry-run -v

# Run a one-shot check
check: build
	./$(BINARY) check

# Validate config
validate: build
	./$(BINARY) validate

# Format code
fmt:
	go fmt ./...
	gofmt -s -w .

# Tidy dependencies
tidy:
	go mod tidy

# Show help
help:
	@echo "WorkloadGuard Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build            Build the binary"
	@echo "  install          Install binary and config"
	@echo "  install-launchd  Install launchd plist for auto-start"
	@echo "  uninstall        Remove binary and launchd plist"
	@echo "  clean            Remove build artifacts"
	@echo "  test             Run tests"
	@echo "  lint             Run golangci-lint"
	@echo "  run              Run in dry-run mode"
	@echo "  check            Run a one-shot check"
	@echo "  validate         Validate config file"
	@echo "  fmt              Format code"
	@echo "  tidy             Tidy go.mod"
