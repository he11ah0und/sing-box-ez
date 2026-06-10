//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
)

// ApplyUpdate downloads the new binary, hands over to a PowerShell script,
// and exits the current process.
func ApplyUpdate(assetURL string, progress func(downloaded, total int64)) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate executable: %w", err)
	}

	tmp := exe + ".tmp"
	if err := DownloadAsset(assetURL, tmp, progress); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

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
