//go:build windows

package giogui

import (
	"os"
	"os/exec"
)

// restartProcess starts a new instance of the same executable and returns
// immediately. The current process is expected to exit afterwards.
func restartProcess(exe string, args []string, env []string) error {
	cmd := exec.Command(exe, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	return cmd.Start()
}

// restartEnv returns a clean environment for the new process.
func restartEnv() []string {
	return os.Environ()
}
