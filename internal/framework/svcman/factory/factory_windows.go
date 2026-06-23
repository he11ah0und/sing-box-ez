//go:build windows

// Package factory detects and returns the most appropriate service manager for
// the current platform.
package factory

import (
	"sing-box-ez/internal/framework/svcman"
	"sing-box-ez/internal/framework/svcman/dummy"
	win "sing-box-ez/internal/framework/svcman/windows"
)

func candidates(serviceName string) []svcman.Manager {
	return []svcman.Manager{
		win.New(serviceName),
		dummy.New(serviceName),
	}
}

// Default returns the best available manager for the current platform.
func Default(serviceName string) (svcman.Manager, error) {
	w := win.New(serviceName)
	if w.Available() {
		return w, nil
	}
	return dummy.New(serviceName), nil
}

// All returns every available service manager for the current platform,
// excluding the dummy fallback.
func All(serviceName string) []svcman.Manager {
	var result []svcman.Manager
	for _, m := range candidates(serviceName) {
		if !m.Available() {
			continue
		}
		if _, ok := m.(*dummy.Manager); ok {
			continue
		}
		result = append(result, m)
	}
	return result
}
