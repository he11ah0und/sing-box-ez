package main

import (
	"log"
	"os"

	"sing-box-ez/internal/cli"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/paths"
)

func printHelp() {
	cli.PrintHelp(os.Stdout)
}

// parseDataDir extracts --data-dir from args and returns the directory + remaining args.
func parseDataDir(args []string) (string, []string) {
	for i := 0; i < len(args); i++ {
		if args[i] == "--data-dir" && i+1 < len(args) {
			dir := args[i+1]
			remaining := append(args[:i], args[i+2:]...)
			return dir, remaining
		}
	}
	return "", args
}

func main() {
	dataDir, args := parseDataDir(os.Args[1:])
	if dataDir != "" {
		paths.SetDataDir(dataDir)
	}
	paths.Init()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if len(args) > 0 {
		if err := cli.Run(args); err != nil {
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
