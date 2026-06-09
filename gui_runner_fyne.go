//go:build !nogui && fyne

package main

import (
	"sing-box-ez/internal/config"
	"sing-box-ez/internal/gui"
)

func runGUI(cfg *config.AppConfig) bool {
	g := gui.New(cfg)
	g.Run()
	return true
}
