//go:build !nogui && !fyne

package main

import (
	"sing-box-ez/internal/config"
	giogui "sing-box-ez/internal/gui/gio"
)

func runGUI(cfg *config.AppConfig) bool {
	g := giogui.New(cfg)
	g.Run()
	return true
}
