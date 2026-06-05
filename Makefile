.PHONY: all build run run-nogui build-nogui clean deps test docs help

APP_NAME := sing-box-ez
BUILD_DIR := ./build
GO := go

VERSION    := $(shell git describe --tags --exact-match 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u +"%Y-%m-%d %H:%M:%S")
BUILD_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# ---------------------------------------------------------------------------
# Build options — override on the command line:
#   make build OS=windows ARCH=arm64 GUI=0
#   make build OS=linux   ARCH=amd64 GUI=1 GUI_BACKEND=wayland
# ---------------------------------------------------------------------------
OS           ?= $(shell go env GOOS)
ARCH         ?= $(shell go env GOARCH)
GUI          ?= 1
GUI_BACKEND  ?= wayland

GOOS    := $(OS)
GOARCH  := $(ARCH)

# On Windows with GUI, hide the console window.
WIN_GUI_FLAG := $(if $(and $(filter windows,$(GOOS)),$(filter 1,$(GUI))),-H windowsgui,)
# Static link MinGW runtime on Windows so no extra DLLs are needed.
WIN_STATIC := $(if $(filter windows,$(GOOS)),-linkmode external -extldflags "-static",)
LDFLAGS := -ldflags "-s -w $(WIN_GUI_FLAG) $(WIN_STATIC) -X 'sing-box-ez/internal/version.Version=$(VERSION)' -X 'sing-box-ez/internal/version.BuildDate=$(BUILD_DATE)' -X 'sing-box-ez/internal/version.Commit=$(BUILD_COMMIT)'"

# Lazy-evaluated variables so target-specific overrides are respected.
# GUI_BACKEND only affects Linux (Wayland vs X11); Windows/macOS use native GLFW.
CGO_ENABLED = $(if $(filter 1,$(GUI)),1,0)
BUILD_TAGS  = $(if $(filter 1,$(GUI)),$(if $(filter linux,$(GOOS)),$(if $(filter wayland,$(GUI_BACKEND)),-tags wayland,),),-tags nogui)
GUI_SUFFIX  = $(if $(filter 1,$(GUI)),$(if $(filter linux,$(GOOS)),$(if $(filter wayland,$(GUI_BACKEND)),-wayland,-x11),),-nogui)
EXT         = $(if $(filter windows,$(GOOS)),.exe,)

# Detect whether we are cross-compiling with CGO enabled.
HOST_OS   := $(shell go env GOOS)
HOST_ARCH := $(shell go env GOARCH)

CROSS_CC :=

ifeq ($(shell [ "$(CGO_ENABLED)" = "1" ] && [ "$(GOOS)-$(GOARCH)" != "$(HOST_OS)-$(HOST_ARCH)" ] && echo 1 || echo 0),1)
  ifeq ($(GOOS),windows)
    ifeq ($(GOARCH),amd64)
      CROSS_CC := x86_64-w64-mingw32-gcc
    endif
  endif
  ifeq ($(GOOS),linux)
    ifeq ($(GOARCH),arm64)
      CROSS_CC := aarch64-linux-gnu-gcc
    endif
  endif
endif

# Explicit user-provided CC takes precedence (ignore plain 'cc'/'gcc'),
# otherwise use auto-detected cross compiler.
BUILD_CC := $(or $(filter-out cc gcc,$(CC)),$(CROSS_CC))

OUTPUT = $(BUILD_DIR)/$(APP_NAME)-$(GOOS)-$(GOARCH)$(GUI_SUFFIX)$(EXT)

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
	@echo "  build       Compile the binary"
	@echo "  build-nogui Alias for 'make build GUI=0'"
	@echo "  run         Build and run locally (GUI mode)"
	@echo "  run-nogui   Build and run locally (CLI mode)"
	@echo "  deps        Download Go dependencies"
	@echo "  test        Run Go tests"
	@echo "  docs        Generate plugin API docs and serve with mkdocs"
	@echo "  defs        Generate VS Code Lua definitions for plugin dev"
	@echo "  clean       Remove build artifacts"
	@echo ""
	@echo "Build options (examples):"
	@echo "  make build                       # native OS/arch, Wayland GUI"
	@echo "  make build GUI=0                 # native OS/arch, CLI only"
	@echo "  make build GUI_BACKEND=x11       # native, X11 GUI"
	@echo "  make build OS=linux ARCH=arm64 GUI=1"
	@echo "  make build OS=windows ARCH=amd64 GUI=0"
	@echo "  make build OS=darwin ARCH=arm64 GUI=1"
	@echo ""
	@echo "Variables:"
	@echo "  OS           Target operating system  (default: current)"
	@echo "  ARCH         Target architecture      (default: current)"
	@echo "  GUI          1 = with GUI (needs CGO), 0 = CLI only"
	@echo "  GUI_BACKEND  wayland | x11  (default: wayland)"
	@echo "  CC           Cross-compiler to use    (auto-detected)"

# ---------------------------------------------------------------------------
# Dependencies & tests
# ---------------------------------------------------------------------------
deps:
	$(GO) mod download
	$(GO) mod tidy

test:
	$(GO) test ./...

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
	@echo "Building: OS=$(GOOS) ARCH=$(GOARCH) GUI=$(GUI) GUI_BACKEND=$(GUI_BACKEND) CGO=$(CGO_ENABLED)"
	$(if $(BUILD_CC),@echo "Cross-compiler: $(BUILD_CC)")
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) \
		$(if $(BUILD_CC),CC=$(BUILD_CC)) $(GO) build $(BUILD_TAGS) $(LDFLAGS) -o $(OUTPUT) .
	@echo "Built: $(OUTPUT)"

# ---------------------------------------------------------------------------
# Convenience aliases
# ---------------------------------------------------------------------------
build-nogui:
	$(MAKE) build GUI=0

# ---------------------------------------------------------------------------
# Run locally
# ---------------------------------------------------------------------------
run: GUI=1
run: build
	$(OUTPUT)

run-nogui: GUI=0
run-nogui: build
	$(OUTPUT)

# ---------------------------------------------------------------------------
# Clean
# ---------------------------------------------------------------------------
clean:
	rm -rf $(BUILD_DIR)
	@echo "Cleaned build directory"
