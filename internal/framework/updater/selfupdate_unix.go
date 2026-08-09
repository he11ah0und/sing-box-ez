//go:build !windows

package updater

import (
	"os"
	"syscall"

	"github.com/he11ah0und/logger"
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
	u.Log.TInfof("updater.self.replacing_binary", exe, newExe)
	if err := os.Chmod(newExe, 0750); err != nil { // #nosec G302 -- replacement binary must remain executable
		return u.Log.TErrorf("updater.self.chmod_replacement_failed", newExe, err)
	}
	if err := os.Rename(newExe, exe); err != nil {
		return u.Log.TErrorf("updater.self.replace_binary_failed", newExe, exe, err)
	}
	return nil
}

// restart replaces the current process by exec'ing the updated binary.
// exe is the current binary path verified by os.Executable();
// os.Args/os.Environ are the process's own context.
// #nosec G702,G204 — safe re-exec of the verified binary with original args.
func (u *unixSelfUpdate) restart(exe string) error {
	u.Log.TInfof("updater.self.restarting_binary", exe)
	return syscall.Exec(exe, os.Args, os.Environ())
}
