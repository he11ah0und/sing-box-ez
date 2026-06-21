//go:build windows

// Package windows implements service management via the Windows Service Control
// Manager using the sc command.
package windows

import (
	"fmt"
	"os/exec"
	"sing-box-ez/internal/svcman"
	"strings"
)

// Manager controls a Windows service.
type Manager struct {
	name string
}

// New creates a Windows service manager.
func New(serviceName string) *Manager {
	return &Manager{name: serviceName}
}

// Name implements svcman.Manager.
func (m *Manager) Name() string {
	return "windows"
}

// Available implements svcman.Manager.
func (m *Manager) Available() bool {
	_, err := exec.LookPath("sc.exe")
	return err == nil
}

// IsInstalled implements svcman.Manager.
func (m *Manager) IsInstalled() bool {
	err := m.sc("query", m.name)
	return err == nil
}

// Install implements svcman.Manager.
func (m *Manager) Install(opts svcman.InstallOptions) error {
	args := []string{"create", m.name, "binPath=", opts.ExecPath, "start=", "auto", "DisplayName=", opts.DisplayName}
	if err := m.sc(args...); err != nil {
		return fmt.Errorf("windows install: %w", err)
	}
	if opts.Description != "" {
		_ = m.sc("description", m.name, opts.Description)
	}
	return nil
}

// Remove implements svcman.Manager.
func (m *Manager) Remove() error {
	_ = m.sc("stop", m.name)
	return m.sc("delete", m.name)
}

// Start implements svcman.Manager.
func (m *Manager) Start() error {
	return m.sc("start", m.name)
}

// Stop implements svcman.Manager.
func (m *Manager) Stop() error {
	return m.sc("stop", m.name)
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
	out, err := exec.Command("sc.exe", "query", m.name).Output()
	if err != nil {
		return svcman.StatusUnknown, nil
	}
	s := strings.ToLower(string(out))
	switch {
	case strings.Contains(s, "running"):
		return svcman.StatusRunning, nil
	case strings.Contains(s, "stopped"):
		return svcman.StatusStopped, nil
	default:
		return svcman.StatusUnknown, nil
	}
}

func (m *Manager) sc(args ...string) error {
	return exec.Command("sc.exe", args...).Run()
}
