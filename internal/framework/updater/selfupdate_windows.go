//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"

	"sing-box-ez/internal/framework/logger"
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

	w.Log.Infof("rotating current binary %q → %q", exe, oldExe)
	_ = os.Remove(oldExe)

	if err := os.Rename(exe, oldExe); err != nil {
		return w.Log.Errorf("rename current binary failed: %v", err)
	}
	if err := os.Rename(newExe, exe); err != nil {
		_ = os.Rename(oldExe, exe)
		return w.Log.Errorf("rename replacement failed: %v", err)
	}
	return nil
}

func (w *windowsSelfUpdate) restart(exe string) error {
	oldExe := exe + ".old"

	w.Log.Infof("starting restart helper for %q", exe)
	script := fmt.Sprintf(`
Start-Sleep -Seconds 1
Remove-Item -Path "%s" -Force -ErrorAction SilentlyContinue
Start-Process -FilePath "%s"
`, oldExe, exe)

	cmd := exec.Command("powershell", "-WindowStyle", "hidden", "-Command", script)
	if err := cmd.Start(); err != nil {
		return w.Log.Errorf("restart script failed: %v", err)
	}

	os.Exit(0)
	return nil
}
