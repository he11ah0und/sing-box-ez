//go:build !nogui

package main

import (
	"fmt"
	"os"
	"runtime"

	"sing-box-ez/internal/config"
	"sing-box-ez/internal/gui"
)

func runGUI(cfg *config.AppConfig) bool {
	// On Linux we need a display server; on Windows/macOS GLFW handles it natively.
	if runtime.GOOS == "linux" {
		hasDisplay := os.Getenv("DISPLAY") != ""
		hasWayland := os.Getenv("WAYLAND_DISPLAY") != ""

		if !hasDisplay && !hasWayland {
			fmt.Fprintln(os.Stderr, "No display server detected (neither DISPLAY nor WAYLAND_DISPLAY set).")
			fmt.Fprintln(os.Stderr, "Please use CLI commands:")
			fmt.Fprintln(os.Stderr)
			printHelp()
			return false
		}

		// Recover from GLFW panics (e.g. platform initialization failure)
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "GUI initialization failed: %v\n", r)
				if hasDisplay && !hasWayland {
					fmt.Fprintln(os.Stderr, "\nThis binary was built for Wayland. To use X11, rebuild with:")
					fmt.Fprintln(os.Stderr, "  make build GUI_BACKEND=x11")
				}
				fmt.Fprintln(os.Stderr)
				printHelp()
			}
		}()
	}

	g := gui.New(cfg)
	g.Run()
	return true
}
