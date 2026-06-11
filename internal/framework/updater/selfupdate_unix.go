//go:build !windows

package updater

import (
	"os"
	"syscall"
)

// defaultRestart replaces the current process by exec'ing the updated binary.
// exe is the current binary path verified by os.Executable();
// os.Args are the process's own arguments. Safe to re-exec.
// #nosec G702,G204 — exe is the current binary path from os.Executable(); os.Args/os.Environ() are the process's own context.
func defaultRestart(exe string) error {
	return syscall.Exec(exe, os.Args, os.Environ())
}
