//go:build !windows

package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func KillProcess(pid int, elevated bool) error {
	if elevated {
		return killTreeElevated(pid)
	}
	killTree(pid)
	return nil
}

func killTreeElevated(pid int) error {
	out, _ := exec.Command("pkexec", "pgrep", "-P", strconv.Itoa(pid)).Output()
	for _, line := range strings.Split(string(out), "\n") {
		if childPid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && childPid > 0 {
			_ = exec.Command("pkexec", "kill", "-9", strconv.Itoa(childPid)).Run()
		}
	}
	return exec.Command("pkexec", "kill", "-9", strconv.Itoa(pid)).Run()
}

func killTree(pid int) {
	out, _ := exec.Command("pgrep", "-P", strconv.Itoa(pid)).Output()
	for _, line := range strings.Split(string(out), "\n") {
		if childPid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && childPid > 0 {
			killTree(childPid)
		}
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

func resolveAbsPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func SetNetAdminCapabilityGUI(path string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	return exec.Command("pkexec", "setcap", "cap_net_admin=+ep", resolveAbsPath(path)).Run()
}

func SetNetAdminCapabilityCLI(path string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	return exec.Command("sudo", "setcap", "cap_net_admin=+ep", resolveAbsPath(path)).Run()
}

func HasNetAdminCapability(path string) bool {
	if runtime.GOOS != "linux" {
		return false
	}
	out, err := exec.Command("getcap", resolveAbsPath(path)).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "cap_net_admin")
}

func ProcessExists(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(os.Signal(nil))
	return err == nil
}

// IsAdmin always returns false on Unix; privilege elevation is handled via pkexec/sudo.
func IsAdmin() bool {
	return false
}
