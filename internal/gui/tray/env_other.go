//go:build !linux && !nogui

package tray

import systray "github.com/gogpu/systray"

func (t *Tray) checkTrayEnvironment() {}

func (t *Tray) refreshTray(tray *systray.SystemTray) {}
