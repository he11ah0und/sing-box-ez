// Package embed provides a svcman.Manager implementation that runs the core
// inside the current process. It is the "embed" mode for sing-box-ez.
package embed

import (
	"sing-box-ez/internal/framework/svcman"
)

// CoreManager is the subset of core.Manager methods required by embed.Manager.
type CoreManager interface {
	Start() error
	Stop() error
	Restart() error
	IsRunning() bool
}

// Manager wraps an in-process core manager so it satisfies svcman.Manager.
type Manager struct {
	name    string
	manager CoreManager
}

// New creates an embed service manager for the given core process manager.
func New(serviceName string, manager CoreManager) *Manager {
	return &Manager{name: serviceName, manager: manager}
}

// Name returns the human-readable backend name.
func (m *Manager) Name() string { return "embed" }

// Available reports whether this backend can be used. The embed backend is
// always available because it does not depend on an external init system.
func (m *Manager) Available() bool { return true }

// IsInstalled always returns true for the embed backend.
func (m *Manager) IsInstalled() bool { return true }

// Install is a no-op for the embed backend.
func (m *Manager) Install(opts svcman.InstallOptions) error { return nil }

// Remove is a no-op for the embed backend.
func (m *Manager) Remove() error { return nil }

// Start starts the in-process core.
func (m *Manager) Start() error { return m.manager.Start() }

// Stop stops the in-process core.
func (m *Manager) Stop() error { return m.manager.Stop() }

// Restart restarts the in-process core.
func (m *Manager) Restart() error { return m.manager.Restart() }

// Status returns the current runtime status of the in-process core.
func (m *Manager) Status() (svcman.Status, error) {
	if m.manager.IsRunning() {
		return svcman.StatusRunning, nil
	}
	return svcman.StatusStopped, nil
}
