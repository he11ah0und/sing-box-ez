package openurl

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
		// #nosec G204 — open is a system binary; URL is parsed and validated above.
		return exec.Command("open", u.String()).Start()
	case "windows":
		// #nosec G204 — cmd is a system binary; URL is parsed and validated above.
		return exec.Command("cmd", "/c", "start", u.String()).Start()
	default:
		// #nosec G204 — xdg-open is a system binary; URL is parsed and validated above.
		return exec.Command("xdg-open", u.String()).Start()
	}
}
