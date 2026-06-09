package util

import (
	"net/url"
	"os/exec"
	"runtime"
)

// OpenURL opens the given URL in the default browser.
func OpenURL(urlStr string) error {
	u, err := url.Parse(urlStr)
	if err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", u.String()).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", u.String()).Start()
	default:
		return exec.Command("xdg-open", u.String()).Start()
	}
}
