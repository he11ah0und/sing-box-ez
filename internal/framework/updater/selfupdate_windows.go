//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
)

// windowsSelfUpdate is the platform backend for replacing the running binary on
// Windows. A running executable cannot be overwritten, so the current binary is
// renamed to exe.old, the replacement takes its place, and a short-lived
// PowerShell helper removes the .old file and starts the new process after this
// one exits.
type windowsSelfUpdate struct{}

func newSelfUpdatePlatform() selfUpdatePlatform { return windowsSelfUpdate{} }

func (windowsSelfUpdate) replace(exe, newExe string) error {
	oldExe := exe + ".old"

	// Remove a leftover .old from a previous update if it exists.
	_ = os.Remove(oldExe)

	if err := os.Rename(exe, oldExe); err != nil {
		return fmt.Errorf("rename current binary: %w", err)
	}
	if err := os.Rename(newExe, exe); err != nil {
		// Try to restore the original binary on failure.
		_ = os.Rename(oldExe, exe)
		return fmt.Errorf("rename replacement: %w", err)
	}
	return nil
}

func (windowsSelfUpdate) restart(exe string) error {
	oldExe := exe + ".old"

	script := fmt.Sprintf(`
Start-Sleep -Seconds 1
Remove-Item -Path "%s" -Force -ErrorAction SilentlyContinue
Start-Process -FilePath "%s"
`, oldExe, exe)

	cmd := exec.Command("powershell", "-WindowStyle", "hidden", "-Command", script)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("restart script failed: %w", err)
	}

	os.Exit(0)
	return nil
}
