//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
)

// defaultRestart hands over to a short-lived PowerShell script that waits for
// this process to exit, replaces the binary, and starts the new one.
func defaultRestart(exe string) error {
	tmp := exe + ".tmp"

	// PowerShell script: wait for us to exit, replace binary, restart.
	script := fmt.Sprintf(`
Start-Sleep -Seconds 1
Move-Item -Path "%s" -Destination "%s" -Force
Start-Process -FilePath "%s"
`, tmp, exe, exe)

	cmd := exec.Command("powershell", "-WindowStyle", "hidden", "-Command", script)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("restart script failed: %w", err)
	}

	os.Exit(0)
	return nil
}
