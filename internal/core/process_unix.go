//go:build !windows

package core

import (
	"errors"
	"fmt"
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
	return killTree(pid)
}

func killTreeElevated(pid int) error {
	out, err := exec.Command("pkexec", "pgrep", "-P", strconv.Itoa(pid)).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			// pgrep returns 1 when no children found — that's OK
		}
	}
	for _, line := range strings.Split(string(out), "\n") {
		if childPid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && childPid > 0 {
			_ = exec.Command("pkexec", "kill", "-9", strconv.Itoa(childPid)).Run()
		}
	}
	return exec.Command("pkexec", "kill", "-9", strconv.Itoa(pid)).Run()
}

func killTree(pid int) error {
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(pid)).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			// pgrep returns 1 when no children found — that's OK
		}
	}
	for _, line := range strings.Split(string(out), "\n") {
		if childPid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && childPid > 0 {
			_ = killTree(childPid)
		}
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}

func resolveAbsPath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Abs(path)
}

func SetNetAdminCapabilityGUI(path string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	absPath, err := resolveAbsPath(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	return exec.Command("pkexec", "setcap", "cap_net_admin=+ep", absPath).Run()
}

func SetNetAdminCapabilityCLI(path string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	absPath, err := resolveAbsPath(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	return exec.Command("sudo", "setcap", "cap_net_admin=+ep", absPath).Run()
}

func HasNetAdminCapability(path string) bool {
	if runtime.GOOS != "linux" {
		return false
	}
	absPath, err := resolveAbsPath(path)
	if err != nil {
		return false
	}
	out, err := exec.Command("getcap", absPath).Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "cap_net_admin")
}

func ProcessExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	_, err := os.Stat(fmt.Sprintf("/proc/%d", pid))
	return err == nil
}

// IsAdmin always returns false on Unix; privilege elevation is handled via pkexec/sudo.
func IsAdmin() bool {
	return false
}
