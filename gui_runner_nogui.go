//go:build nogui

package main

import (
	"fmt"
	"os"

	"sing-box-ez/internal/app"
)

func runGUI(app *app.App) bool {
	fmt.Println("GUI mode is not available in this build.")
	fmt.Println("Please use CLI commands:")
	fmt.Println("")
	app.CLI.PrintHelp(os.Stdout)
	return false
}
