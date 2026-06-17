//go:build !windows

package giogui

import (
	"errors"

	gioapp "gioui.org/app"
	"gioui.org/io/system"
)

// hideMainWindow hides or minimizes the main window on platforms without a
// native hide/show implementation. It is used as a fallback.
func hideMainWindow(w *gioapp.Window) error {
	if w == nil {
		return errors.New("no window")
	}
	w.Perform(system.ActionMinimize)
	return nil
}

// showMainWindow restores and raises the main window on platforms without a
// native hide/show implementation. It is used as a fallback.
func showMainWindow(w *gioapp.Window) error {
	if w == nil {
		return errors.New("no window")
	}
	w.Option(gioapp.Windowed.Option())
	w.Perform(system.ActionRaise)
	return nil
}
