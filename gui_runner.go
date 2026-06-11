//go:build !nogui && !fyne

package main

import (
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework"
	giogui "sing-box-ez/internal/gui/gio"
)

func runGUI(cfg *config.AppConfig, fwApp *framework.App) bool {
	g := giogui.New(cfg, fwApp)
	g.Run()
	return true
}
