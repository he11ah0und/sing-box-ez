package pages

import (
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"gio.tools/icons"

	"sing-box-ez/internal/core"
	"sing-box-ez/internal/framework/localengine"
)

// PluginsPage is a placeholder for the plugins screen.
type PluginsPage struct {
	th   *material.Theme
	ctrl *core.Controller
}

// NewPluginsPage creates a new plugins page.
func NewPluginsPage(th *material.Theme, ctrl *core.Controller) *PluginsPage {
	return &PluginsPage{th: th, ctrl: ctrl}
}

// Tag returns the page tag.
func (p *PluginsPage) Tag() string { return "plugins" }

// Name returns the page name.
func (p *PluginsPage) Name() string { return localengine.T("tab", "plugins") }
func (p *PluginsPage) Icon() *widget.Icon { return icons.ActionExtension }

// Layout draws the plugins page.
func (p *PluginsPage) Layout(gtx layout.Context) layout.Dimensions {
	return material.Body1(p.th, "Plugins Page").Layout(gtx)
}
