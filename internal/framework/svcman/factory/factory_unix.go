//go:build !windows

// Package factory detects and returns the most appropriate service manager for
// the current platform.
package factory

import (
	"fmt"
	"os"
	"os/exec"

	"sing-box-ez/internal/framework/svcman"
	"sing-box-ez/internal/framework/svcman/dummy"
	"sing-box-ez/internal/framework/svcman/openrc"
	"sing-box-ez/internal/framework/svcman/systemd"
	"sing-box-ez/internal/framework/svcman/sysv"
)

func candidates(serviceName string) []svcman.Manager {
	return []svcman.Manager{
		systemd.NewSystem(serviceName),
		systemd.NewUser(serviceName),
		openrc.New(serviceName),
		sysv.New(serviceName),
		dummy.New(serviceName),
	}
}

// Default returns the best available manager for the current platform.
func Default(serviceName string) (svcman.Manager, error) {
	for _, m := range candidates(serviceName) {
		if !m.Available() {
			continue
		}
		// systemd user services require a user manager; skip if not present.
		if sm, ok := m.(*systemd.Manager); ok && sm.UserMode() {
			if os.Getuid() == 0 {
				continue
			}
			if _, err := exec.LookPath("systemctl"); err != nil {
				continue
			}
		}
		return m, nil
	}

	return nil, fmt.Errorf("no service manager available for %q", serviceName)
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
