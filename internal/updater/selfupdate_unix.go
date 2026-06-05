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

	if err := os.Chmod(tmp, 0755); err != nil {
		return err
	}

	if err := os.Rename(tmp, exe); err != nil {
		return fmt.Errorf("replace failed: %w", err)
	}

	return syscall.Exec(exe, os.Args, os.Environ())
}
