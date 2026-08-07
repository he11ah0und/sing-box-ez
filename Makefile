.PHONY: all build build-nogui run run-nogui dev clean deps test vet fmt fmt-check lint ineffassign-check security complexity outdated analyze help

APP_NAME := sing-box-ez
BUILD_DIR := ./build
GO := go
WAILS3 := $(shell go env GOPATH)/bin/wails3
GOPATH := $(shell go env GOPATH)
GO_BIN := $(GOPATH)/bin

# Private he11ah0und modules are fetched directly via git, bypassing
# proxy.golang.org and the checksum database.
export GOPRIVATE := github.com

BRANCH     := $(shell git branch --show-current 2>/dev/null || git describe --tags --exact-match 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
BUILD_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DEV    := $(shell git diff-index --quiet HEAD 2>/dev/null && echo "false" || echo "true")
COMMIT_DATE  := $(shell git log -1 --format=%cI 2>/dev/null || echo "unknown")

# ---------------------------------------------------------------------------
# Build options — override on the command line:
#   make build OS=windows ARCH=amd64
#   make build-nogui OS=linux ARCH=amd64
# ---------------------------------------------------------------------------
OS       ?= $(shell go env GOOS)
ARCH     ?= $(shell go env GOARCH)
COMPILER ?= gcc
PLUGINS  ?= 1

GOOS   := $(OS)
GOARCH := $(ARCH)

# On Windows with GUI, hide the console window.
WIN_GUI_FLAG := $(if $(filter windows,$(GOOS)),-H windowsgui,)
LDFLAGS := -s -w $(WIN_GUI_FLAG) \
	-X 'sing-box-ez/internal/framework/version.Branch=$(BRANCH)' \
	-X 'sing-box-ez/internal/framework/version.BuildDate=$(BUILD_DATE)' \
	-X 'sing-box-ez/internal/framework/version.Commit=$(BUILD_COMMIT)' \
	-X 'sing-box-ez/internal/framework/version.BuildOS=$(GOOS)' \
	-X 'sing-box-ez/internal/framework/version.BuildArch=$(GOARCH)' \
	-X 'sing-box-ez/internal/framework/version.BuildGUI=1' \
	-X 'sing-box-ez/internal/framework/version.BuildCompiler=$(COMPILER)' \
	-X 'sing-box-ez/internal/framework/version.BuildDev=$(BUILD_DEV)' \
	-X 'sing-box-ez/internal/framework/version.CommitDate=$(COMMIT_DATE)'

NOGUI_LDFLAGS := -s -w \
	-X 'sing-box-ez/internal/framework/version.Branch=$(BRANCH)' \
	-X 'sing-box-ez/internal/framework/version.BuildDate=$(BUILD_DATE)' \
	-X 'sing-box-ez/internal/framework/version.Commit=$(BUILD_COMMIT)' \
	-X 'sing-box-ez/internal/framework/version.BuildOS=$(GOOS)' \
	-X 'sing-box-ez/internal/framework/version.BuildArch=$(GOARCH)' \
	-X 'sing-box-ez/internal/framework/version.BuildGUI=0' \
	-X 'sing-box-ez/internal/framework/version.BuildCompiler=$(COMPILER)' \
	-X 'sing-box-ez/internal/framework/version.BuildDev=$(BUILD_DEV)' \
	-X 'sing-box-ez/internal/framework/version.CommitDate=$(COMMIT_DATE)'

comma := ,
empty :=
space := $(empty) $(empty)
TAG_LIST := $(if $(filter 0,$(PLUGINS)),noplugins,)
BUILD_TAGS := $(if $(strip $(TAG_LIST)),-tags "$(subst $(space),$(comma),$(strip $(TAG_LIST)))",)

NOGUI_TAG_LIST := nogui$(if $(filter 0,$(PLUGINS)), noplugins,)
NOGUI_BUILD_TAGS := -tags "$(subst $(space),$(comma),$(strip $(NOGUI_TAG_LIST)))"

EXT             := $(if $(filter windows,$(GOOS)),.exe,)
COMPILER_SUFFIX := -$(COMPILER)
GUI_OUTPUT      := $(BUILD_DIR)/$(APP_NAME)-$(GOARCH)-$(GOOS)-$(COMPILER)-gui$(EXT)
CLI_OUTPUT      := $(BUILD_DIR)/$(APP_NAME)-$(GOARCH)-$(GOOS)-$(COMPILER)-cli$(EXT)

# Wails v3 uses webkitgtk-6.0 / WebKit2GTK-4.1 on Linux; no extra tag needed.

# ---------------------------------------------------------------------------
# Default target
# ---------------------------------------------------------------------------
all: build

# ---------------------------------------------------------------------------
# Help
# ---------------------------------------------------------------------------
help:
	@echo "Usage: make <target> [options]"
	@echo ""
	@echo "Targets:"
	@echo "  build              Compile the Wails GUI binary"
	@echo "  build-nogui        Compile the CLI-only binary"
	@echo "  run                Build and run locally (GUI mode)"
	@echo "  run-nogui          Build and run locally (CLI mode)"
	@echo "  dev                Run Wails in development mode"
	@echo "  deps               Download and tidy Go dependencies"
	@echo "  test               Run Go tests"
	@echo "  vet                Run go vet"
	@echo "  fmt                Format Go source files"
	@echo "  fmt-check          Check Go formatting without changing files"
	@echo "  lint               Run staticcheck"
	@echo "  ineffassign-check  Run ineffassign"
	@echo "  security           Run gosec security scanner"
	@echo "  complexity         Run gocyclo complexity check"
	@echo "  outdated           List outdated Go modules"
	@echo "  analyze            Run fmt-check, vet, test, lint, ineffassign-check, complexity and security"
	@echo "  proto              Generate protobuf Go bindings"
	@echo "  schema             Generate sing-box schema YAML"
	@echo "  docs               Generate and serve documentation"
	@echo "  defs               Generate plugin definitions"
	@echo "  setup              Install system dependencies (Debian/Ubuntu)"
	@echo "  setup-arch         Install system dependencies and Go analysis tools (Arch Linux)"
	@echo "  clean              Remove build artifacts"
	@echo ""
	@echo "Build options (examples):"
	@echo "  make build                       # native OS/arch GUI"
	@echo "  make build-nogui                 # native OS/arch CLI"
	@echo "  make build OS=windows ARCH=amd64 # cross-compile Windows GUI"
	@echo "  make build OS=linux ARCH=arm64   # cross-compile Linux GUI"
	@echo ""
	@echo "Variables:"
	@echo "  OS          Target operating system  (default: current)"
	@echo "  ARCH        Target architecture      (default: current)"
	@echo "  COMPILER    gcc | musl                (default: gcc)"
	@echo "  PLUGINS     1 = with plugins, 0 = without  (default: 1)"

# ---------------------------------------------------------------------------
# System dependencies
# ---------------------------------------------------------------------------
setup:
	@command -v apt-get >/dev/null 2>&1 || { echo "apt-get not found. Install dependencies manually."; exit 0; }
ifeq ($(COMPILER),musl)
	@echo "Installing musl build dependencies..."
	sudo apt-get update -qq
	sudo apt-get install --no-install-recommends -y musl-tools
else
	@echo "Installing Wails v3 build dependencies..."
	sudo apt-get update -qq
	sudo apt-get install --no-install-recommends -y gcc libgtk-4-dev libwebkitgtk-6.0-dev
endif

setup-arch:
	@command -v pacman >/dev/null 2>&1 || { echo "pacman not found. This target is for Arch Linux only."; exit 0; }
	@echo "Installing Wails v3 build dependencies..."
	sudo pacman -S --needed gtk4 webkitgtk-6.0
	@echo "Installing Go analysis tools..."
	go install honnef.co/go/tools/cmd/staticcheck@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	go install github.com/gordonklaus/ineffassign@latest
	go install github.com/segmentio/golines@latest
	go install github.com/fzipp/gocyclo/cmd/gocyclo@latest

# ---------------------------------------------------------------------------
# Dependencies & tests
# ---------------------------------------------------------------------------
deps:
	$(GO) mod download
	$(GO) mod tidy

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

# ---------------------------------------------------------------------------
# Code quality & analysis
# ---------------------------------------------------------------------------
fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || { echo "Unformatted files:"; gofmt -l .; exit 1; }

lint:
	$(GO_BIN)/staticcheck ./...

ineffassign-check:
	$(GO_BIN)/ineffassign ./...

security:
	$(GO_BIN)/gosec -quiet -exclude-dir=internal/core/api/singbox/proto ./...

complexity:
	$(GO_BIN)/gocyclo -over 15 .

outdated:
	$(GO) list -m -u all | grep '\['

analyze: fmt-check vet test lint ineffassign-check complexity security
	@echo "Full analysis complete"

# ---------------------------------------------------------------------------
# Generated code
# ---------------------------------------------------------------------------
proto:
	$(GO) generate ./internal/core/api/singbox/...

schema:
	$(GO) run ./cmd/schema-gen -repo-dir /tmp/sing-box-schema-repo -out internal/singboxconfig/schema.yaml

# ---------------------------------------------------------------------------
# Docs
# ---------------------------------------------------------------------------
docs:
	$(GO) run . docs
	mkdocs serve

defs:
	$(GO) run . defs

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
build:
	@mkdir -p $(BUILD_DIR)
	@echo "Building: OS=$(GOOS) ARCH=$(GOARCH) GUI=1"
	@echo "Installing frontend dependencies..."
	cd frontend && npm install --omit=none
	@echo "Generating Wails v3 bindings..."
	$(WAILS3) generate bindings -clean=true -ts -i
	@echo "Building frontend..."
	cd frontend && npm run build
	@echo "Building GUI binary..."
	CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) build -trimpath -buildvcs=false -ldflags "$(LDFLAGS)" -o $(GUI_OUTPUT) .
	@echo "Built: $(GUI_OUTPUT)"

build-nogui:
	@mkdir -p $(BUILD_DIR)
	@echo "Building: OS=$(GOOS) ARCH=$(GOARCH) GUI=0"
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(GO) build -trimpath -buildvcs=false $(NOGUI_BUILD_TAGS) -ldflags "$(NOGUI_LDFLAGS)" -o $(CLI_OUTPUT) .
	@echo "Built: $(CLI_OUTPUT)"

# ---------------------------------------------------------------------------
# Development & run
# ---------------------------------------------------------------------------
dev:
	PATH=$(GO_BIN):$(PATH) $(WAILS3) dev -config ./build/config.yml

run: build
	$(GUI_OUTPUT) $(ARGS)

run-nogui: build-nogui
	$(CLI_OUTPUT) $(ARGS)

# ---------------------------------------------------------------------------
# Clean
# ---------------------------------------------------------------------------
clean:
	rm -rf $(BUILD_DIR)
	@echo "Cleaned build directory"
