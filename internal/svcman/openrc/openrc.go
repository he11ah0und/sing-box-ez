// Package openrc implements service management via OpenRC.
package openrc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sing-box-ez/internal/svcman"
	"strings"
)

// Manager controls an OpenRC service.
type Manager struct {
	name string
	path string
}

// New creates an OpenRC manager.
func New(serviceName string) *Manager {
	return &Manager{
		name: serviceName,
		path: filepath.Join("/etc", "init.d", serviceName),
	}
}

// Name implements svcman.Manager.
func (m *Manager) Name() string {
	return "openrc"
}

// Available implements svcman.Manager.
func (m *Manager) Available() bool {
	_, err1 := exec.LookPath("rc-service")
	_, err2 := os.Stat("/etc/init.d")
	return err1 == nil && err2 == nil
}

// IsInstalled implements svcman.Manager.
func (m *Manager) IsInstalled() bool {
	_, err := os.Stat(m.path)
	return err == nil
}

// Install implements svcman.Manager.
func (m *Manager) Install(opts svcman.InstallOptions) error {
	script := buildScript(m.name, opts)
	if err := os.WriteFile(m.path, []byte(script), 0o755); err != nil {
		return fmt.Errorf("openrc install: %w", err)
	}
	return exec.Command("rc-update", "add", m.name, "default").Run()
}

// Remove implements svcman.Manager.
func (m *Manager) Remove() error {
	_ = exec.Command("rc-update", "del", m.name, "default").Run()
	return os.Remove(m.path)
}

// Start implements svcman.Manager.
func (m *Manager) Start() error {
	return exec.Command("rc-service", m.name, "start").Run()
}

// Stop implements svcman.Manager.
func (m *Manager) Stop() error {
	return exec.Command("rc-service", m.name, "stop").Run()
}

// Restart implements svcman.Manager.
func (m *Manager) Restart() error {
	return exec.Command("rc-service", m.name, "restart").Run()
}

// Status implements svcman.Manager.
func (m *Manager) Status() (svcman.Status, error) {
	out, err := exec.Command("rc-service", m.name, "status").Output()
	if err != nil {
		return svcman.StatusUnknown, nil
	}
	s := strings.ToLower(string(out))
	if strings.Contains(s, "started") {
		return svcman.StatusRunning, nil
	}
	return svcman.StatusStopped, nil
}

func buildScript(name string, opts svcman.InstallOptions) string {
	execLine := opts.ExecPath
	if len(opts.Args) > 0 {
		execLine += " " + strings.Join(opts.Args, " ")
	}
	return fmt.Sprintf(`#!/sbin/openrc-run

description="%s"
command="%s"
command_args="%s"
pidfile="/run/%s.pid"
command_background=true
`, opts.DisplayName, opts.ExecPath, strings.Join(opts.Args, " "), name)
}
