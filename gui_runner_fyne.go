//go:build !nogui && fyne

package main

import (
	"sing-box-ez/internal/app"
	fynegui "sing-box-ez/internal/gui/fyne"
)

func runGUI(app *app.App) bool {
	g := fynegui.New(app)
	g.Run()
	return true
}
