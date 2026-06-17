//go:build !windows

package giogui

import (
	"os"
	"syscall"
)

// restartProcess replaces the current process with a new instance of the same
// executable, preserving the original command-line arguments and environment.
func restartProcess(exe string, args []string, env []string) error {
	return syscall.Exec(exe, args, env)
}

// restartEnv returns a clean environment for the new process.
func restartEnv() []string {
	return os.Environ()
}
