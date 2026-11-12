//go:build windows

package openurl

import (
	"os/exec"
	"syscall"
)

// hideConsole prevents the console window that `cmd /c start` would otherwise
// flash on Windows.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
