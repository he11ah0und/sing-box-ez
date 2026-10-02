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
	"time"
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

// Soft-stop timing: give the process time to clean up (sing-box tears down
// tun ip rules and nftables redirects on SIGTERM), then escalate to SIGKILL
// if it is still alive.
const (
	stopWaitStep  = 100 * time.Millisecond
	stopWaitLimit = 5 * time.Second
)

// signalTree sends sig to the process group led by pid (children are placed
// there via setProcessGroup), falling back to the process itself when it is
// not a group leader. An already-gone target (ESRCH) is not an error, so
// stopping a dead process stays idempotent.
func signalTree(pid int, sig syscall.Signal) error {
	if err := syscall.Kill(-pid, sig); err != nil {
		if !errors.Is(err, syscall.ESRCH) {
			return err
		}
		if err := syscall.Kill(pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
			return err
		}
	}
	return nil
}

// waitProcessGone polls until pid disappears or limit elapses. It returns true
// when the process is gone.
func waitProcessGone(pid int, limit time.Duration) bool {
	deadline := time.Now().Add(limit)
	for ProcessExists(pid) {
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(stopWaitStep)
	}
	return true
}

// pkexecKillGroup signals the process group led by pid through pkexec. A
// vanished target is treated as success so elevated stops are idempotent.
func pkexecKillGroup(pid int, sig string) error {
	// #nosec G204 — pkexec and kill are system binaries; pid is a validated process ID.
	// The "--" separator keeps kill from parsing the negative process-group id as an option.
	err := exec.Command("pkexec", "kill", sig, "--", "-"+strconv.Itoa(pid)).Run()
	if err != nil && !ProcessExists(pid) {
		return nil
	}
	return err
}

func killTreeElevated(pid int) error {
	if !ProcessExists(pid) {
		return nil
	}
	if err := pkexecKillGroup(pid, "-TERM"); err != nil {
		return err
	}
	if waitProcessGone(pid, stopWaitLimit) {
		return nil
	}
	return pkexecKillGroup(pid, "-9")
}

func killTree(pid int) error {
	if !ProcessExists(pid) {
		return nil
	}
	if err := signalTree(pid, syscall.SIGTERM); err != nil {
		return err
	}
	if waitProcessGone(pid, stopWaitLimit) {
		return nil
	}
	return signalTree(pid, syscall.SIGKILL)
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
	out, err := exec.Command("pkexec", "setcap", "cap_net_admin=+ep", absPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("pkexec setcap: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func RemoveNetAdminCapabilityGUI(path string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	absPath, err := resolveAbsPath(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	// #nosec G204 — pkexec and setcap are system binaries; absPath is resolved internal path.
	out, err := exec.Command("pkexec", "setcap", "-r", absPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("pkexec setcap -r: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
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
