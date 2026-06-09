//go:build fyne && !noplugins

package plugins

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// UIBuilder allows Lua scripts to construct simple Fyne UI.
type UIBuilder struct {
	window fyne.Window
	tabs   *container.AppTabs
}

// NewUIBuilder creates a UI builder bound to the given window and tab container.
func NewUIBuilder(w fyne.Window, tabs *container.AppTabs) *UIBuilder {
	return &UIBuilder{window: w, tabs: tabs}
}
