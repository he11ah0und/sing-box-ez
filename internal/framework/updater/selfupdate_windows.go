//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/he11ah0und/logger"
)

// windowsSelfUpdate is the platform backend for replacing the running binary on
// Windows. A running executable cannot be overwritten, so the current binary is
// renamed to exe.old, the replacement takes its place, and a short-lived
// PowerShell helper removes the .old file and starts the new process after this
// one exits.
type windowsSelfUpdate struct {
	Log *logger.LogTerminal
}

func newSelfUpdatePlatform(parent *logger.LogTerminal) selfUpdatePlatform {
	return &windowsSelfUpdate{Log: parent.Allocate("platform")}
}

func (w *windowsSelfUpdate) replace(exe, newExe string) error {
	oldExe := exe + ".old"

	w.Log.TInfof("updater.self.rotating_binary", exe, oldExe)
	_ = os.Remove(oldExe)

	if err := os.Rename(exe, oldExe); err != nil {
		return w.Log.TErrorf("updater.self.rename_current_failed", err)
	}
	if err := os.Rename(newExe, exe); err != nil {
		_ = os.Rename(oldExe, exe)
		return w.Log.TErrorf("updater.self.rename_replacement_failed", err)
	}
	return nil
}

func (w *windowsSelfUpdate) restart(exe string) error {
	oldExe := exe + ".old"

	w.Log.TInfof("updater.self.starting_restart_helper", exe)
	script := fmt.Sprintf(`
Start-Sleep -Seconds 1
Remove-Item -Path "%s" -Force -ErrorAction SilentlyContinue
Start-Process -FilePath "%s"
`, oldExe, exe)

	cmd := exec.Command("powershell", "-WindowStyle", "hidden", "-Command", script)
	if err := cmd.Start(); err != nil {
		return w.Log.TErrorf("updater.self.restart_script_failed", err)
	}

	os.Exit(0)
	return nil
}
