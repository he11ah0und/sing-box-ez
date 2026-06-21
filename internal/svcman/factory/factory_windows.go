//go:build windows

// Package factory detects and returns the most appropriate service manager for
// the current platform.
package factory

import (
	"sing-box-ez/internal/svcman"
	"sing-box-ez/internal/svcman/dummy"
	win "sing-box-ez/internal/svcman/windows"
)

// Default returns the best available manager for the current platform.
func Default(serviceName string) (svcman.Manager, error) {
	w := win.New(serviceName)
	if w.Available() {
		return w, nil
	}
	return dummy.New(serviceName), nil
}
