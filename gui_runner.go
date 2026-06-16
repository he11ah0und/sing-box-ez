//go:build !nogui && !fyne

package main

import (
	"sing-box-ez/internal/app"
	giogui "sing-box-ez/internal/gui/gio"
)

func runGUI(app *app.App) bool {
	g := giogui.New(app)
	g.Run()
	return true
}
