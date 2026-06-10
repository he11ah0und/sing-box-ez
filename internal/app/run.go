// Package app contains the top-level application bootstrap logic.
// It wires together CLI parsing, data directory setup, config loading,
// and the decision to run in CLI or GUI mode.
package app

import (
	"log"
	"os"

	"sing-box-ez/internal/cli"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework/util/paths"
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
func Run(args []string, runGUI func(*config.AppConfig) bool) {
	dataDir, remaining := parseDataDir(args)
	if dataDir != "" {
		paths.SetDataDir(dataDir)
	}
	paths.Init()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if len(remaining) > 0 {
		if err := cli.Run(remaining); err != nil {
			log.Fatalf("CLI error: %v", err)
		}
		return
	}

	if !runGUI(cfg) {
		// GUI could not start (no display, no wayland, etc).
		// Exit gracefully so make/run does not show a scary error.
		os.Exit(0)
	}
}
