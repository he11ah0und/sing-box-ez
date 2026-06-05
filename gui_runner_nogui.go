//go:build nogui

package main

import (
	"fmt"

	"sing-box-ez/internal/config"
)

func runGUI(cfg *config.AppConfig) bool {
	fmt.Println("GUI mode is not available in this build.")
	fmt.Println("Please use CLI commands:")
	fmt.Println("")
	printHelp()
	return false
}
