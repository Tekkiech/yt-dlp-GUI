# Makefile for yt-dlp-GUI (macOS-only)
# Usage:
#   make build           # build native binary for current host arch
#   make build-amd64     # build Intel macOS binary
#   make build-arm64     # build Apple Silicon binary
#   make build-universal # build fat/universal binary (requires lipo)
#   make tidy            # run go mod tidy
#   make clean           # remove dist artifacts
#   make help            # show this help

BINARY ?= yt-dlp-gui
PKG ?= .
OUTDIR ?= dist
GO ?= go
LIPO ?= lipo
CODESIGN ?= codesign

# Extra build flags
# Example: make BUILD_FLAGS="-ldflags='-s -w'" build
BUILD_FLAGS ?=

# Default GO settings for static-ish build (CGO disabled)
CGO_ENABLED ?= 0

# Arch-specific output names
OUT_AMD64 := $(OUTDIR)/$(BINARY)-darwin-amd64
OUT_ARM64 := $(OUTDIR)/$(BINARY)-darwin-arm64
OUT_UNIV := $(OUTDIR)/$(BINARY)-darwin-universal

.PHONY: help tidy build build-amd64 build-arm64 build-universal clean release deps install-deps

help:
	@echo "Makefile targets:"
	@echo "  make build           Build native binary for the current host arch"
	@echo "  make build-amd64     Build Intel (amd64) macOS binary"
	@echo "  make build-arm64     Build Apple Silicon (arm64) macOS binary"
	@echo "  make build-universal Build universal (fat) macOS binary (requires lipo)"
	@echo "  make tidy            Run 'go mod tidy' in repo root"
	@echo "  make clean           Remove $(OUTDIR)"
	@echo "  make release         Build universal and create tar.gz in $(OUTDIR)"
	@echo "  make deps            Print helpful dependency install commands (Homebrew)"
	@echo ""

# Ensure output dir exists and run go mod tidy first by default for destructive targets
tidy:
	@echo "Running go mod tidy..."
	@$(GO) mod tidy

build: tidy
	@echo "Building native binary for host..."
	@mkdir -p $(OUTDIR)
	@$(GO) build -trimpath $(BUILD_FLAGS) -o "$(OUTDIR)/$(BINARY)" $(PKG)
	@echo "Built: $(OUTDIR)/$(BINARY)"

build-amd64: tidy
	@echo "Building macOS Intel (amd64) binary..."
	@mkdir -p $(OUTDIR)
	@env GOOS=darwin GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) $(GO) build -trimpath $(BUILD_FLAGS) -o "$(OUT_AMD64)" $(PKG)
	@echo "Built: $(OUT_AMD64)"

build-arm64: tidy
	@echo "Building macOS Apple Silicon (arm64) binary..."
	@mkdir -p $(OUTDIR)
	@env GOOS=darwin GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) $(GO) build -trimpath $(BUILD_FLAGS) -o "$(OUT_ARM64)" $(PKG)
	@echo "Built: $(OUT_ARM64)"

build-universal: build-amd64 build-arm64
	@echo "Creating universal (fat) binary with $(LIPO)..."
	@if ! command -v $(LIPO) >/dev/null 2>&1; then \
		echo "Error: $(LIPO) not found. Install Xcode command line tools or ensure lipo is available."; \
		exit 1; \
	fi
	@mkdir -p $(OUTDIR)
	@$(LIPO) -create -output "$(OUT_UNIV)" "$(OUT_AMD64)" "$(OUT_ARM64)"
	@echo "Universal binary created: $(OUT_UNIV)"
	@# Optionally codesign (developer to uncomment and supply identity)
	@if command -v $(CODESIGN) >/dev/null 2>&1; then \
		echo "Note: codesign available. You may sign the binary before distribution (optional)."; \
	fi

release: build-universal
	@echo "Creating release archive..."
	@mkdir -p $(OUTDIR)
	@tar -C $(OUTDIR) -czf $(OUTDIR)/$(BINARY)-darwin-universal.tar.gz "$(notdir $(OUT_UNIV))"
	@echo "Release archive: $(OUTDIR)/$(BINARY)-darwin-universal.tar.gz"

clean:
	@echo "Removing $(OUTDIR)..."
	@rm -rf $(OUTDIR)
	@echo "clean complete."

deps:
	@echo "Recommended macOS runtime/developer packages (Homebrew):"
	@echo "  brew install yt-dlp ffmpeg"
	@echo "If you plan to codesign or notarize, install Xcode and Xcode command-line tools."
	@echo "lipo is provided by Xcode command-line tools and is required for universal builds."

# Convenience: build for host when no target provided
.DEFAULT_GOAL := build
