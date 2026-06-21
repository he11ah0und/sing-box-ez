//go:build !windows

// Package factory detects and returns the most appropriate service manager for
// the current platform.
package factory

import (
	"fmt"
	"os"
	"os/exec"

	"sing-box-ez/internal/svcman"
	"sing-box-ez/internal/svcman/dummy"
	"sing-box-ez/internal/svcman/openrc"
	"sing-box-ez/internal/svcman/systemd"
	"sing-box-ez/internal/svcman/sysv"
)

// Default returns the best available manager for the current platform.
func Default(serviceName string) (svcman.Manager, error) {
	candidates := []svcman.Manager{
		systemd.NewSystem(serviceName),
		systemd.NewUser(serviceName),
		openrc.New(serviceName),
		sysv.New(serviceName),
		dummy.New(serviceName),
	}

	// Prefer a real init system when running as root or when systemctl is
	// available and systemd is actually running.
	for _, m := range candidates {
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
