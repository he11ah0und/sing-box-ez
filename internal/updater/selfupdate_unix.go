//go:build !windows

package updater

import (
	"fmt"
	"os"
	"syscall"
)

// ApplyUpdate replaces the current binary and restarts the process.
func ApplyUpdate(assetURL string, progress func(downloaded, total int64)) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate executable: %w", err)
	}

	tmp := exe + ".tmp"
	if err := DownloadAsset(assetURL, tmp, progress); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// #nosec G302 — tmp is the replacement binary for the current executable; must remain executable.
	if err := os.Chmod(tmp, 0750); err != nil {
		return err
	}

	if err := os.Rename(tmp, exe); err != nil {
		return fmt.Errorf("replace failed: %w", err)
	}

	// exe is the current binary path verified by os.Executable();
	// os.Args are the process's own arguments. Safe to re-exec.
	// #nosec G702,G204 — exe is the current binary path from os.Executable(); os.Args/os.Environ() are the process's own context.
	return syscall.Exec(exe, os.Args, os.Environ())
}
