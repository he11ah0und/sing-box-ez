package framework

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultDataDir returns the conventional user data directory for appName.
// On Windows it uses %APPDATA%/%LOCALAPPDATA% with a fallback to %TEMP%.
// On Unix-like systems it uses ~/.{appName}.
func DefaultDataDir(appName string) string {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = os.Getenv("LOCALAPPDATA")
		}
		if appData == "" {
			appData = os.TempDir()
		}
		return filepath.Join(appData, appName)
	default:
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		return filepath.Join(home, "."+appName)
	}
}
