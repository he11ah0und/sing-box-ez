//go:build !nogui

package main

import (
	"embed"

	apppkg "sing-box-ez/internal/app"
	"sing-box-ez/internal/gui/wails"
)

//go:embed all:frontend/dist
var assets embed.FS

func runGUI(app *apppkg.App) bool {
	w := wails.New(app, assets)
	if err := w.Run(); err != nil {
		// Errors are already logged by Wails or the backend.
		return false
	}
	return true
}
