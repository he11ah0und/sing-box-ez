// Package sysv implements service management via SysVinit /etc/init.d scripts.
package sysv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sing-box-ez/internal/svcman"
	"strings"
)

// Manager controls a SysVinit service.
type Manager struct {
	name string
	path string
}

// New creates a SysVinit manager.
func New(serviceName string) *Manager {
	return &Manager{
		name: serviceName,
		path: filepath.Join("/etc", "init.d", serviceName),
	}
}

// Name implements svcman.Manager.
func (m *Manager) Name() string {
	return "sysvinit"
}

// Available implements svcman.Manager.
func (m *Manager) Available() bool {
	_, err := os.Stat("/etc/init.d")
	return err == nil
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
		return fmt.Errorf("sysv install: %w", err)
	}
	for _, level := range []string{"2", "3", "4", "5"} {
		target := filepath.Join("/etc", "rc"+level+".d", "S99"+m.name)
		_ = os.Symlink(m.path, target)
	}
	return nil
}

// Remove implements svcman.Manager.
func (m *Manager) Remove() error {
	for _, level := range []string{"2", "3", "4", "5"} {
		_ = os.Remove(filepath.Join("/etc", "rc"+level+".d", "S99"+m.name))
	}
	return os.Remove(m.path)
}

// Start implements svcman.Manager.
func (m *Manager) Start() error {
	return m.run("start")
}

// Stop implements svcman.Manager.
func (m *Manager) Stop() error {
	return m.run("stop")
}

// Restart implements svcman.Manager.
func (m *Manager) Restart() error {
	return m.run("restart")
}

// Status implements svcman.Manager.
func (m *Manager) Status() (svcman.Status, error) {
	out, err := m.runOutput("status")
	if err != nil {
		return svcman.StatusStopped, nil
	}
	s := strings.ToLower(string(out))
	if strings.Contains(s, "running") {
		return svcman.StatusRunning, nil
	}
	return svcman.StatusStopped, nil
}

func (m *Manager) run(arg string) error {
	return exec.Command("service", m.name, arg).Run()
}

func (m *Manager) runOutput(arg string) ([]byte, error) {
	return exec.Command("service", m.name, arg).Output()
}

func buildScript(name string, opts svcman.InstallOptions) string {
	execLine := opts.ExecPath
	if len(opts.Args) > 0 {
		execLine += " " + strings.Join(opts.Args, " ")
	}
	return fmt.Sprintf(`#!/bin/sh
### BEGIN INIT INFO
# Provides:          %s
# Required-Start:    $network $remote_fs
# Required-Stop:     $network $remote_fs
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: %s
### END INIT INFO

DAEMON="%s"
PIDFILE="/var/run/%s.pid"

case "$1" in
  start)
    start-stop-daemon --start --background --make-pidfile --pidfile "$PIDFILE" --exec "$DAEMON"
    ;;
  stop)
    start-stop-daemon --stop --pidfile "$PIDFILE"
    ;;
  restart)
    $0 stop && $0 start
    ;;
  status)
    if [ -f "$PIDFILE" ] && kill -0 $(cat "$PIDFILE") 2>/dev/null; then
      echo "running"
    else
      echo "stopped"
    fi
    ;;
  *)
    echo "Usage: $0 {start|stop|restart|status}"
    exit 1
    ;;
esac
exit 0
`, name, opts.DisplayName, execLine, name)
}
