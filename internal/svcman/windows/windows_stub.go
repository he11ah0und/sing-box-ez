//go:build !windows

// Package windows stubs out the Windows service manager on non-Windows
// platforms so that the package is always importable.
package windows

import (
	"errors"
	"sing-box-ez/internal/svcman"
)

// Manager is a no-op stub on non-Windows platforms.
type Manager struct{}

// New returns a stub manager.
func New(serviceName string) *Manager {
	return &Manager{}
}

// Name implements svcman.Manager.
func (m *Manager) Name() string { return "windows" }

// Available implements svcman.Manager.
func (m *Manager) Available() bool { return false }

// IsInstalled implements svcman.Manager.
func (m *Manager) IsInstalled() bool { return false }

// Install implements svcman.Manager.
func (m *Manager) Install(opts svcman.InstallOptions) error {
	return errors.New("Windows services are only supported on Windows")
}

// Remove implements svcman.Manager.
func (m *Manager) Remove() error { return errors.New("Windows services are only supported on Windows") }

// Start implements svcman.Manager.
func (m *Manager) Start() error { return errors.New("Windows services are only supported on Windows") }

// Stop implements svcman.Manager.
func (m *Manager) Stop() error { return errors.New("Windows services are only supported on Windows") }

// Restart implements svcman.Manager.
func (m *Manager) Restart() error {
	return errors.New("Windows services are only supported on Windows")
}

// Status implements svcman.Manager.
func (m *Manager) Status() (svcman.Status, error) {
	return svcman.StatusUnknown, errors.New("Windows services are only supported on Windows")
}
