# Aether Makefile (cross-platform; on Windows use `ming32-make` or scripts\build.ps1)
BINARY_NAME=aether
GO=go
BUILD_DIR=bin
# Single source of truth for the version (see internal/version/version.go).
VERSION=$(shell cat VERSION 2>/dev/null || echo dev)
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS=-s -w
VERSION_LDFLAGS=-X github.com/Debajyoti0-0/aether/internal/version.Version=$(VERSION) -X github.com/Debajyoti0-0/aether/internal/version.Commit=$(COMMIT)

# `rm -rf`/`mkdir -p` replaced by Go's own tooling so this works on
# Windows cmd, macOS, and every Linux distro with make+go only.
RM = go run ./scripts/rmdir
CLEAN_DIR = $(BUILD_DIR)

.PHONY: all build clean test test-race vet lint ci build-all \
        build-windows-amd64 build-windows-arm64 \
        build-linux-amd64 build-linux-arm64 \
        build-darwin-amd64 build-darwin-arm64 \
        completions man

all: build

build:
	$(GO) build -ldflags="$(LDFLAGS) $(VERSION_LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)$(EXT) ./cmd/aether

clean:
	-$(GO) run ./scripts/rmdir $(BUILD_DIR)

test:
	$(GO) test ./...

test-race:
	$(GO) test -race -count=1 ./...

vet:
	$(GO) vet ./...

lint:
	golangci-lint run

# Local CI gate: mirrors .github/workflows/ci.yml.
ci: vet test-race

# --- Per-platform binaries (Kali, macOS, Windows, BSD, ARM SBCs) ---
EXT :=

build-windows-amd64:
	EXT=.exe GOOS=windows GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS) $(VERSION_LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/aether

build-windows-arm64:
	EXT=.exe GOOS=windows GOARCH=arm64 $(GO) build -ldflags="$(LDFLAGS) $(VERSION_LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe ./cmd/aether

build-linux-amd64:
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS) $(VERSION_LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/aether

build-linux-arm64:
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags="$(LDFLAGS) $(VERSION_LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/aether

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags="$(LDFLAGS) $(VERSION_LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/aether

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags="$(LDFLAGS) $(VERSION_LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/aether

build-all: build-linux-amd64 build-linux-arm64 build-darwin-amd64 build-darwin-arm64 build-windows-amd64 build-windows-arm64

# Shell completions (bash/zsh/fish/powershell) — installed per OS docs.
completions:
	$(GO) run ./cmd/aether completion bash > $(BUILD_DIR)/aether.bash
	$(GO) run ./cmd/aether completion zsh > $(BUILD_DIR)/_aether
	$(GO) run ./cmd/aether completion fish > $(BUILD_DIR)/aether.fish
	$(GO) run ./cmd/aether completion powershell > $(BUILD_DIR)/aether.ps1

# Man page (cobra GenManTree) — see scripts/genman.
man:
	$(GO) run ./scripts/genman $(BUILD_DIR)

# SBOM generation (requires syft: https://github.com/anchore/syft)
sbom:
	syft $(BUILD_DIR)/$(BINARY_NAME) -o cyclonedx-json > aether.sbom.json
	@echo "SBOM written to aether.sbom.json"

# Vulnerability scan (requires govulncheck: go install golang.org/x/vuln/cmd/govulncheck@latest)
scan:
	govulncheck ./...

# Release signing (requires cosign; keyless via OIDC or key-based)
sign:
	cosign sign-blob --key cosign.key $(BUILD_DIR)/$(BINARY_NAME) --output-signature $(BUILD_DIR)/$(BINARY_NAME).sig
	@echo "Signature written to $(BUILD_DIR)/$(BINARY_NAME).sig"

deb: build-linux-amd64
	@echo "Run: dpkg-deb --build deploy/debian ../aether_$(VERSION)_amd64.deb"
