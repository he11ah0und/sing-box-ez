package pages

import (
	"gioui.org/layout"
	"gioui.org/widget/material"

	"sing-box-ez/internal/core"
)

// CorePage is a placeholder for the core management screen.
type CorePage struct {
	th   *material.Theme
	ctrl *core.Controller
}

// NewCorePage creates a new core page.
func NewCorePage(th *material.Theme, ctrl *core.Controller) *CorePage {
	return &CorePage{th: th, ctrl: ctrl}
}

// Tag returns the page tag.
func (p *CorePage) Tag() string { return "core" }

// Name returns the page name.
func (p *CorePage) Name() string { return "Core" }

// Layout draws the core page.
func (p *CorePage) Layout(gtx layout.Context) layout.Dimensions {
	return material.Body1(p.th, "Core Page").Layout(gtx)
}
