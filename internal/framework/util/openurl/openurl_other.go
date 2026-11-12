//go:build !windows

package openurl

import "os/exec"

// hideConsole is a no-op outside Windows.
func hideConsole(*exec.Cmd) {}
