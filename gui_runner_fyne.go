//go:build !nogui && fyne

package main

import (
	"sing-box-ez/internal/config"
	fynegui "sing-box-ez/internal/gui/fyne"
)

func runGUI(cfg *config.AppConfig) bool {
	g := fynegui.New(cfg)
	g.Run()
	return true
}
