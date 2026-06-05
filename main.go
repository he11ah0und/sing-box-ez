package main

import (
	"log"
	"os"

	"sing-box-ez/internal/cli"
	"sing-box-ez/internal/config"
)

func printHelp() {
	cli.PrintHelp(os.Stdout)
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if len(os.Args) > 1 {
		if err := cli.Run(os.Args[1:]); err != nil {
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


