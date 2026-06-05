//go:build windows

package core

import (
	"os/exec"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
}

func setNoWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}
}

func KillProcess(pid int, elevated bool) error {
	// On Windows taskkill /F forces termination regardless of elevation
	// for processes owned by the same user. The elevated flag is kept
	// for API consistency with the Unix implementation.
	_ = elevated
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}

func SetNetAdminCapabilityGUI(path string) error {
	return nil
}

func SetNetAdminCapabilityCLI(path string) error {
	return nil
}

func HasNetAdminCapability(path string) bool {
	return false
}

func ProcessExists(pid int) bool {
	const PROCESS_QUERY_INFORMATION = 0x0400
	handle, err := syscall.OpenProcess(PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	syscall.CloseHandle(handle)
	return true
}

// IsAdmin reports whether the current process is running with administrator privileges.
func IsAdmin() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		return false
	}
	return member
}
