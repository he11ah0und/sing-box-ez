// Package dummy provides an in-memory service manager for development and
// testing. It does not integrate with the operating system.
package dummy

import (
	"fmt"
	"os"
	"path/filepath"
	"sing-box-ez/internal/framework/svcman"
	"strconv"
	"sync"
)

// Manager is a development-only manager that tracks state in memory and via a
// PID file.
type Manager struct {
	name    string
	pidFile string
	mu      sync.Mutex
}

// New creates a new dummy manager.
func New(serviceName string) *Manager {
	return &Manager{
		name:    serviceName,
		pidFile: filepath.Join(os.TempDir(), serviceName+".pid"),
	}
}

// Name implements svcman.Manager.
func (m *Manager) Name() string {
	return "dummy"
}

// Available implements svcman.Manager.
func (m *Manager) Available() bool {
	return true
}

// IsInstalled implements svcman.Manager.
func (m *Manager) IsInstalled() bool {
	_, err := os.Stat(m.pidFile)
	return err == nil
}

// Install implements svcman.Manager.
func (m *Manager) Install(opts svcman.InstallOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := os.Create(m.pidFile); err != nil {
		return fmt.Errorf("dummy install: %w", err)
	}
	return nil
}

// Remove implements svcman.Manager.
func (m *Manager) Remove() error {
	return os.Remove(m.pidFile)
}

// Start implements svcman.Manager.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return os.WriteFile(m.pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644)
}

// Stop implements svcman.Manager.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return os.Remove(m.pidFile)
}

// Restart implements svcman.Manager.
func (m *Manager) Restart() error {
	if err := m.Stop(); err != nil {
		return err
	}
	return m.Start()
}

// Status implements svcman.Manager.
func (m *Manager) Status() (svcman.Status, error) {
	_, err := os.Stat(m.pidFile)
	if err != nil {
		return svcman.StatusStopped, nil
	}
	return svcman.StatusRunning, nil
}
