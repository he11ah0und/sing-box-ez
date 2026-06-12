//go:build !windows

package updater

import (
	"os"
	"syscall"

	"sing-box-ez/internal/framework/logger"
)

// unixSelfUpdate is the platform backend for replacing the running binary on
// Unix-like systems. The running executable can be overwritten directly because
// the kernel keeps the old inode mapped until the process exits.
type unixSelfUpdate struct {
	Log *logger.LogTerminal
}

func newSelfUpdatePlatform(parent *logger.LogTerminal) selfUpdatePlatform {
	return &unixSelfUpdate{Log: parent.Allocate("platform")}
}

func (u *unixSelfUpdate) replace(exe, newExe string) error {
	u.Log.Infof("replacing running binary %q with %q", exe, newExe)
	if err := os.Chmod(newExe, 0750); err != nil {
		return u.Log.Errorf("chmod replacement %q failed: %v", newExe, err)
	}
	if err := os.Rename(newExe, exe); err != nil {
		return u.Log.Errorf("replace binary %q → %q failed: %v", newExe, exe, err)
	}
	return nil
}

// restart replaces the current process by exec'ing the updated binary.
// exe is the current binary path verified by os.Executable();
// os.Args/os.Environ are the process's own context.
// #nosec G702,G204 — safe re-exec of the verified binary with original args.
func (u *unixSelfUpdate) restart(exe string) error {
	u.Log.Infof("restarting updated binary %q", exe)
	return syscall.Exec(exe, os.Args, os.Environ())
}
