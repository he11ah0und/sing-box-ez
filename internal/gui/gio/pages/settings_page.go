package pages

import (
	"gioui.org/layout"
	"gioui.org/widget/material"

	"sing-box-ez/internal/core"
)

// SettingsPage is a placeholder for the settings screen.
type SettingsPage struct {
	th   *material.Theme
	ctrl *core.Controller
}

// NewSettingsPage creates a new settings page.
func NewSettingsPage(th *material.Theme, ctrl *core.Controller) *SettingsPage {
	return &SettingsPage{th: th, ctrl: ctrl}
}

// Tag returns the page tag.
func (p *SettingsPage) Tag() string { return "settings" }

// Name returns the page name.
func (p *SettingsPage) Name() string { return "Settings" }

// Layout draws the settings page.
func (p *SettingsPage) Layout(gtx layout.Context) layout.Dimensions {
	return material.Body1(p.th, "Settings Page").Layout(gtx)
}
