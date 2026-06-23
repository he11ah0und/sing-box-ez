// Package systemd implements service management via systemctl.
package systemd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sing-box-ez/internal/framework/svcman"
	"strings"
)

// Manager controls a systemd user or system service.
type Manager struct {
	name string
	user bool
	dir  string
	unit string
}

// NewUser creates a per-user systemd manager.
func NewUser(serviceName string) *Manager {
	home, _ := os.UserHomeDir()
	return &Manager{
		name: serviceName,
		user: true,
		dir:  filepath.Join(home, ".config", "systemd", "user"),
		unit: serviceName + ".service",
	}
}

// NewSystem creates a system-wide systemd manager.
func NewSystem(serviceName string) *Manager {
	return &Manager{
		name: serviceName,
		user: false,
		dir:  "/etc/systemd/system",
		unit: serviceName + ".service",
	}
}

// UserMode reports whether this manager targets the per-user systemd instance.
func (m *Manager) UserMode() bool {
	return m.user
}

// Name implements svcman.Manager.
func (m *Manager) Name() string {
	if m.user {
		return "systemd-user"
	}
	return "systemd"
}

// Available implements svcman.Manager.
func (m *Manager) Available() bool {
	_, err := exec.LookPath("systemctl")
	return err == nil
}

func (m *Manager) unitFilePath() string {
	return filepath.Join(m.dir, m.unit)
}

// IsInstalled implements svcman.Manager.
func (m *Manager) IsInstalled() bool {
	_, err := os.Stat(m.unitFilePath())
	return err == nil
}

// Install implements svcman.Manager.
func (m *Manager) Install(opts svcman.InstallOptions) error {
	if err := os.MkdirAll(m.dir, 0o755); err != nil {
		return fmt.Errorf("systemd install: %w", err)
	}

	unit := buildUnit(opts)
	if err := os.WriteFile(m.unitFilePath(), []byte(unit), 0o644); err != nil {
		return fmt.Errorf("systemd install: write unit: %w", err)
	}

	if err := m.systemctl("daemon-reload"); err != nil {
		return err
	}
	return m.systemctl("enable", m.unit)
}

// Remove implements svcman.Manager.
func (m *Manager) Remove() error {
	_ = m.systemctl("disable", m.unit)
	if err := os.Remove(m.unitFilePath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return m.systemctl("daemon-reload")
}

// Start implements svcman.Manager.
func (m *Manager) Start() error {
	return m.systemctl("start", m.unit)
}

// Stop implements svcman.Manager.
func (m *Manager) Stop() error {
	return m.systemctl("stop", m.unit)
}

// Restart implements svcman.Manager.
func (m *Manager) Restart() error {
	return m.systemctl("restart", m.unit)
}

// Status implements svcman.Manager.
func (m *Manager) Status() (svcman.Status, error) {
	out, err := m.systemctlOutput("is-active", m.unit)
	state := strings.TrimSpace(string(out))
	switch state {
	case "active":
		return svcman.StatusRunning, nil
	case "inactive", "failed":
		return svcman.StatusStopped, nil
	default:
		if err != nil {
			return svcman.StatusUnknown, nil
		}
		return svcman.StatusUnknown, nil
	}
}

func (m *Manager) systemctl(args ...string) error {
	cmd := m.cmd(args...)
	return cmd.Run()
}

func (m *Manager) systemctlOutput(args ...string) ([]byte, error) {
	cmd := m.cmd(args...)
	return cmd.Output()
}

func (m *Manager) cmd(args ...string) *exec.Cmd {
	if m.user {
		args = append([]string{"--user"}, args...)
	}
	return exec.Command("systemctl", args...)
}

func buildUnit(opts svcman.InstallOptions) string {
	execLine := opts.ExecPath
	if len(opts.Args) > 0 {
		execLine += " " + strings.Join(opts.Args, " ")
	}
	userLine := ""
	if opts.User != "" {
		userLine = fmt.Sprintf("User=%s\nGroup=%s\n", opts.User, opts.User)
	}
	workDirLine := ""
	if opts.WorkDir != "" {
		workDirLine = fmt.Sprintf("WorkingDirectory=%s\n", opts.WorkDir)
	}
	return fmt.Sprintf(`[Unit]
Description=%s
After=network-online.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=5
%s%s
[Install]
WantedBy=default.target
`, opts.DisplayName, execLine, userLine, workDirLine)
}
