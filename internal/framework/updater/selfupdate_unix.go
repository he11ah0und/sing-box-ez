//go:build !windows

package updater

import (
	"fmt"
	"os"
	"syscall"
)

// unixSelfUpdate is the platform backend for replacing the running binary on
// Unix-like systems. The running executable can be overwritten directly because
// the kernel keeps the old inode mapped until the process exits.
type unixSelfUpdate struct{}

func newSelfUpdatePlatform() selfUpdatePlatform { return unixSelfUpdate{} }

func (unixSelfUpdate) replace(exe, newExe string) error {
	if err := os.Chmod(newExe, 0750); err != nil {
		return fmt.Errorf("chmod replacement: %w", err)
	}
	if err := os.Rename(newExe, exe); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}
	return nil
}

// restart replaces the current process by exec'ing the updated binary.
// exe is the current binary path verified by os.Executable();
// os.Args/os.Environ are the process's own context.
// #nosec G702,G204 — safe re-exec of the verified binary with original args.
func (unixSelfUpdate) restart(exe string) error {
	return syscall.Exec(exe, os.Args, os.Environ())
}
