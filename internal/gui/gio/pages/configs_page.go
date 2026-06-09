package pages

import (
	"gioui.org/layout"
	"gioui.org/widget/material"

	"sing-box-ez/internal/core"
)

// ConfigsPage is a placeholder for the configs screen.
type ConfigsPage struct {
	th   *material.Theme
	ctrl *core.Controller
}

// NewConfigsPage creates a new configs page.
func NewConfigsPage(th *material.Theme, ctrl *core.Controller) *ConfigsPage {
	return &ConfigsPage{th: th, ctrl: ctrl}
}

// Tag returns the page tag.
func (p *ConfigsPage) Tag() string { return "configs" }

// Name returns the page name.
func (p *ConfigsPage) Name() string { return "Configs" }

// Layout draws the configs page.
func (p *ConfigsPage) Layout(gtx layout.Context) layout.Dimensions {
	return material.Body1(p.th, "Configs Page").Layout(gtx)
}
