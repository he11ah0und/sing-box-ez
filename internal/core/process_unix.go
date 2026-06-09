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

func setNoWindow(cmd *exec.Cmd) {
	// no-op on Unix
}

func KillProcess(pid int, elevated bool) error {
	if elevated {
		return killTreeElevated(pid)
	}
	return killTree(pid)
}

func killTreeElevated(pid int) error {
	// #nosec G204 — pkexec and pgrep are system binaries; pid is validated process ID.
	out, err := exec.Command("pkexec", "pgrep", "-P", strconv.Itoa(pid)).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			// pgrep returns 1 when no children found — that's OK
		}
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if childPid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil && childPid > 0 {
			// #nosec G204 — pkexec and kill are system binaries; childPid is validated from pgrep output.
			_ = exec.Command("pkexec", "kill", "-9", strconv.Itoa(childPid)).Run()
		}
	}
	// #nosec G204 — pkexec and kill are system binaries; pid is validated process ID.
	return exec.Command("pkexec", "kill", "-9", strconv.Itoa(pid)).Run()
}

func killTree(pid int) error {
	// #nosec G204 — pgrep is a system binary; pid is a validated process ID.
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(pid)).Output()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			// pgrep returns 1 when no children found — that's OK
		}
	}
	for line := range strings.SplitSeq(string(out), "\n") {
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
	// #nosec G204 — pkexec and setcap are system binaries; absPath is resolved internal path.
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
	// #nosec G204 — sudo and setcap are system binaries; absPath is resolved internal path.
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
	// #nosec G204 — getcap is a system binary; absPath is resolved internal path.
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
