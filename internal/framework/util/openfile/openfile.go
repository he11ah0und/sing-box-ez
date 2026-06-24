// Package openfile opens a local file with the platform default application.
package openfile

import (
	"os/exec"
	"runtime"
)

// OpenPath opens the given file path using the OS default handler.
func OpenPath(path string) error {
	switch runtime.GOOS {
	case "darwin":
		// #nosec G204 — open is a system binary; path is an internal config file.
		return exec.Command("open", path).Start()
	case "windows":
		// #nosec G204 — cmd is a system binary; path is an internal config file.
		return exec.Command("cmd", "/c", "start", "", path).Start()
	default:
		// #nosec G204 — xdg-open is a system binary; path is an internal config file.
		return exec.Command("xdg-open", path).Start()
	}
}
