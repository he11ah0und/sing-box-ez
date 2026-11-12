//go:build !windows

package openfile

import "os/exec"

// hideConsole is a no-op outside Windows.
func hideConsole(*exec.Cmd) {}
