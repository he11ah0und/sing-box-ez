// Package app contains the top-level application bootstrap logic.
// It wires together CLI parsing, data directory setup, config loading,
// and the decision to run in CLI or GUI mode.
package app

import (
	"embed"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"sing-box-ez/internal/cli"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework"
)

// parseDataDir extracts --data-dir from args and returns the directory + remaining args.
func parseDataDir(args []string) (string, []string) {
	for i := range args {
		if args[i] == "--data-dir" && i+1 < len(args) {
			dir := args[i+1]
			remaining := append(args[:i], args[i+2:]...)
			return dir, remaining
		}
	}
	return "", args
}

// Run bootstraps the application: parses CLI flags, initializes paths,
// loads config, and either runs a CLI command or starts the GUI.
// runGUI receives both the loaded config and the framework App so the GUI can
// wire framework-level services into the core controller.
func defaultDataDir() string {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = os.Getenv("LOCALAPPDATA")
		}
		if appData == "" {
			appData = os.TempDir()
		}
		return filepath.Join(appData, "sing-box-ez")
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		return filepath.Join(home, ".sing-box-ez")
	}
}

func Run(args []string, localesFS embed.FS, runGUI func(*config.AppConfig, *framework.App) bool) {
	dataDir, remaining := parseDataDir(args)
	if dataDir == "" {
		dataDir = defaultDataDir()
	}
	_ = os.MkdirAll(dataDir, 0750)
	_ = os.MkdirAll(filepath.Join(dataDir, "configs"), 0750)
	_ = os.MkdirAll(filepath.Join(dataDir, "plugins"), 0750)
	_ = os.MkdirAll(filepath.Join(dataDir, "docs"), 0750)

	core.BaseDir = dataDir

	cfg, err := config.Load(dataDir)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	app := NewApp(cfg, localesFS)

	if len(remaining) > 0 {
		if err := cli.Run(remaining, dataDir); err != nil {
			log.Fatalf("CLI error: %v", err)
		}
		return
	}

	if !runGUI(cfg, app.App) {
		// GUI could not start (no display, no wayland, etc).
		// Exit gracefully so make/run does not show a scary error.
		os.Exit(0)
	}
}
