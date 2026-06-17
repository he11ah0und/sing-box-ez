//go:build windows && !nogui

package tray

// waitForDone returns immediately on Windows.
//
// gogpu/systray's Windows implementation runs a GetMessage loop that is not
// unblocked by Remove()/DestroyWindow, so waiting on the done channel would
// hang forever and prevent the application from exiting. The tray goroutine
// will be terminated when the process exits.
func waitForDone(t *Tray) {
	// Intentionally empty: do not block shutdown on the tray message loop.
}
