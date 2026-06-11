//go:build !nogui && fyne

package main

import (
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/framework"
	fynegui "sing-box-ez/internal/gui/fyne"
)

func runGUI(cfg *config.AppConfig, fwApp *framework.App) bool {
	g := fynegui.New(cfg, fwApp)
	g.Run()
	return true
}
