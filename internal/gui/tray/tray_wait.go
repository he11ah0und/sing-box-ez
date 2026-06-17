//go:build !windows && !nogui

package tray

// waitForDone blocks until the tray message loop goroutine signals completion.
// On platforms where gogpu/systray's Remove/Destroy unblocks Run (Linux/macOS),
// this waits on the done channel.
func waitForDone(t *Tray) {
	<-t.done
}
