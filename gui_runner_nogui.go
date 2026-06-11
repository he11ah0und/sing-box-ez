//go:build nogui

package main

import (
	"fmt"
	"os"

	"sing-box-ez/internal/cli"
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework"
)

func runGUI(cfg *config.AppConfig, fwApp *framework.App) bool {
	fmt.Println("GUI mode is not available in this build.")
	fmt.Println("Please use CLI commands:")
	fmt.Println("")
	cli.PrintHelp(os.Stdout)
	return false
}
